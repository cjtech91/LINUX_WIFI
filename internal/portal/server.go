package portal

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"errors"
	"html/template"
	"net"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"linux_wifi/internal/gpio"
	"linux_wifi/internal/store"
)

type ServerDeps struct {
	Store             *store.Store
	Allowlister       Allowlister
	DefaultMinutes    int
	Title             string
	AdminUser         string
	AdminPass         string
	CoinCounter       gpio.PulseCounter
	CoinPesoPerPulse  int
	CoinWindowSeconds int
}

type Server struct {
	store            *store.Store
	allowlister      Allowlister
	defaultMinutes   int
	title            string
	portalTmpl       *template.Template
	adminUser        string
	adminPass        string
	adminTmpl        *template.Template
	coinCounter      gpio.PulseCounter
	coinPesoPerPulse int
	coinWindow       time.Duration

	coinMu   sync.Mutex
	coinSess *coinSession
}

func NewServer(d ServerDeps) *Server {
	allowlister := d.Allowlister
	if allowlister == nil {
		allowlister = NoopAllowlister{}
	}
	mins := d.DefaultMinutes
	if mins <= 0 {
		mins = 60
	}
	title := strings.TrimSpace(d.Title)
	if title == "" {
		title = "PiSoWiFi"
	}

	tmpl := template.Must(template.New("portal").Parse(portalHTML))
	admin := template.Must(template.New("admin").Parse(adminHTML))

	counter := d.CoinCounter
	if counter == nil {
		counter = gpio.DisabledPulseCounter{}
	}
	pesoPerPulse := d.CoinPesoPerPulse
	if pesoPerPulse <= 0 {
		pesoPerPulse = 1
	}
	windowSec := d.CoinWindowSeconds
	if windowSec <= 0 {
		windowSec = 60
	}

	return &Server{
		store:            d.Store,
		allowlister:      allowlister,
		defaultMinutes:   mins,
		title:            title,
		portalTmpl:       tmpl,
		adminUser:        strings.TrimSpace(d.AdminUser),
		adminPass:        strings.TrimSpace(d.AdminPass),
		adminTmpl:        admin,
		coinCounter:      counter,
		coinPesoPerPulse: pesoPerPulse,
		coinWindow:       time.Duration(windowSec) * time.Second,
	}
}

func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /", s.handlePortal)
	mux.HandleFunc("POST /login", s.handlePortalLogin)

	mux.HandleFunc("POST /api/v1/vouchers", s.handleCreateVoucher)
	mux.HandleFunc("POST /api/v1/login", s.handleAPILogin)
	mux.HandleFunc("GET /api/v1/status", s.handleStatus)
	mux.HandleFunc("GET /api/v1/rates", s.handleClientRates)
	mux.HandleFunc("POST /api/v1/coin/start", s.handleCoinStart)
	mux.HandleFunc("GET /api/v1/coin/status", s.handleCoinStatus)
	mux.HandleFunc("POST /api/v1/coin/commit", s.handleCoinCommit)
	mux.HandleFunc("POST /api/v1/coin/cancel", s.handleCoinCancel)

	mux.HandleFunc("GET /admin", s.handleAdmin)
	mux.HandleFunc("GET /api/admin/summary", s.handleAdminSummary)
	mux.HandleFunc("GET /api/admin/interfaces", s.handleAdminInterfaces)
	mux.HandleFunc("GET /api/admin/vouchers", s.handleAdminVouchers)
	mux.HandleFunc("GET /api/admin/logs", s.handleAdminLogs)
	mux.HandleFunc("GET /api/admin/rates", s.handleAdminGetRates)
	mux.HandleFunc("POST /api/admin/rates", s.handleAdminSetRates)
	mux.HandleFunc("GET /api/admin/subvendo/devices", s.handleAdminSubVendoDevices)
}

func (s *Server) handlePortal(w http.ResponseWriter, r *http.Request) {
	clientIP := clientIPFromRequest(r)
	redir := r.URL.Query().Get("redir")
	data := struct {
		Title    string
		ClientIP string
		Redir    string
		Error    string
		Ok       string
	}{
		Title:    s.title,
		ClientIP: clientIP,
		Redir:    redir,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = s.portalTmpl.Execute(w, data)
}

func (s *Server) handlePortalLogin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	code := strings.TrimSpace(r.Form.Get("code"))
	redir := strings.TrimSpace(r.Form.Get("redir"))
	clientIP := clientIPFromRequest(r)

	ctx := r.Context()
	res, err := s.consumeVoucher(ctx, code, "", clientIP)
	if err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		data := struct {
			Title    string
			ClientIP string
			Redir    string
			Error    string
			Ok       string
		}{
			Title:    s.title,
			ClientIP: clientIP,
			Redir:    redir,
			Error:    err.Error(),
		}
		_ = s.portalTmpl.Execute(w, data)
		return
	}

	if redir == "" {
		redir = "http://example.com/"
	}
	http.Redirect(w, r, redir, http.StatusSeeOther)
	_ = res
}

type createVoucherRequest struct {
	Minutes int `json:"minutes"`
}

type createVoucherResponse struct {
	Code    string `json:"code"`
	Minutes int    `json:"minutes"`
}

func (s *Server) handleCreateVoucher(w http.ResponseWriter, r *http.Request) {
	var req createVoucherRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	minutes := req.Minutes
	if minutes <= 0 {
		minutes = s.defaultMinutes
	}

	code, err := newVoucherCode(8)
	if err != nil {
		http.Error(w, "failed to generate voucher", http.StatusInternalServerError)
		return
	}
	v, err := s.store.CreateVoucher(r.Context(), code, minutes)
	if err != nil {
		http.Error(w, "failed to create voucher", http.StatusInternalServerError)
		return
	}

	writeJSON(w, createVoucherResponse{
		Code:    v.Code,
		Minutes: v.Minutes,
	})
}

type loginRequest struct {
	Code string `json:"code"`
	MAC  string `json:"mac"`
}

type loginResponse struct {
	IP       string    `json:"ip"`
	MAC      string    `json:"mac,omitempty"`
	EndAtUTC time.Time `json:"end_at_utc"`
}

func (s *Server) handleAPILogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	clientIP := clientIPFromRequest(r)
	res, err := s.consumeVoucher(r.Context(), strings.TrimSpace(req.Code), strings.TrimSpace(req.MAC), clientIP)
	if err != nil {
		if errors.Is(err, store.ErrVoucherNotFound) || errors.Is(err, store.ErrVoucherUsed) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, "login failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, loginResponse{
		IP:       clientIP,
		MAC:      strings.TrimSpace(req.MAC),
		EndAtUTC: res.Session.EndAt.UTC(),
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	ip := strings.TrimSpace(r.URL.Query().Get("ip"))
	if ip == "" {
		ip = clientIPFromRequest(r)
	}
	now := time.Now().UTC()
	sess, ok, err := s.store.GetActiveSessionByIP(r.Context(), ip, now)
	if err != nil {
		http.Error(w, "status failed", http.StatusInternalServerError)
		return
	}
	type statusResponse struct {
		IP          string    `json:"ip"`
		Active      bool      `json:"active"`
		EndAtUTC    time.Time `json:"end_at_utc,omitempty"`
		SecondsLeft int64     `json:"seconds_left,omitempty"`
	}
	resp := statusResponse{
		IP:     ip,
		Active: ok,
	}
	if ok {
		resp.EndAtUTC = sess.EndAt.UTC()
		secondsLeft := int64(sess.EndAt.Sub(now).Truncate(time.Second).Seconds())
		if secondsLeft < 0 {
			secondsLeft = 0
		}
		resp.SecondsLeft = secondsLeft
	}
	writeJSON(w, resp)
}

type rate struct {
	Minutes  int     `json:"minutes"`
	Price    float64 `json:"price"`
	UpMbps   float64 `json:"up_mbps,omitempty"`
	DownMbps float64 `json:"down_mbps,omitempty"`
	Pause    bool    `json:"pause,omitempty"`
}

type coinSession struct {
	Token     string
	ClientIP  string
	StartAt   time.Time
	ExpiresAt time.Time
	BaseCount uint64
}

func (s *Server) handleClientRates(w http.ResponseWriter, r *http.Request) {
	rates := s.getRates(r.Context())
	writeJSON(w, struct {
		Items []rate `json:"items"`
	}{
		Items: rates,
	})
}

func (s *Server) handleCoinStart(w http.ResponseWriter, r *http.Request) {
	if !s.coinCounter.Enabled() {
		http.Error(w, "coin disabled", http.StatusBadRequest)
		return
	}
	now := time.Now().UTC()
	clientIP := clientIPFromRequest(r)

	s.coinMu.Lock()
	defer s.coinMu.Unlock()

	if s.coinSess != nil && now.Before(s.coinSess.ExpiresAt) {
		secondsLeft := int64(s.coinSess.ExpiresAt.Sub(now).Truncate(time.Second).Seconds())
		if secondsLeft < 0 {
			secondsLeft = 0
		}
		w.WriteHeader(http.StatusConflict)
		writeJSON(w, struct {
			Ok          bool  `json:"ok"`
			Busy        bool  `json:"busy"`
			SecondsLeft int64 `json:"seconds_left"`
		}{
			Ok:          false,
			Busy:        true,
			SecondsLeft: secondsLeft,
		})
		return
	}

	tok, err := newVoucherCode(16)
	if err != nil {
		http.Error(w, "coin start failed", http.StatusInternalServerError)
		return
	}
	s.coinSess = &coinSession{
		Token:     tok,
		ClientIP:  clientIP,
		StartAt:   now,
		ExpiresAt: now.Add(s.coinWindow),
		BaseCount: s.coinCounter.Current(),
	}

	writeJSON(w, struct {
		Ok            bool      `json:"ok"`
		Token         string    `json:"token"`
		ExpiresAtUTC  time.Time `json:"expires_at_utc"`
		PesoPerPulse  int       `json:"peso_per_pulse"`
		WindowSeconds int       `json:"window_seconds"`
		ClientIP      string    `json:"client_ip"`
	}{
		Ok:            true,
		Token:         tok,
		ExpiresAtUTC:  s.coinSess.ExpiresAt,
		PesoPerPulse:  s.coinPesoPerPulse,
		WindowSeconds: int(s.coinWindow.Seconds()),
		ClientIP:      clientIP,
	})
}

func (s *Server) handleCoinStatus(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		http.Error(w, "missing token", http.StatusBadRequest)
		return
	}
	now := time.Now().UTC()

	s.coinMu.Lock()
	sess := s.coinSess
	s.coinMu.Unlock()

	if sess == nil || sess.Token != token {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	secondsLeft := int64(sess.ExpiresAt.Sub(now).Truncate(time.Second).Seconds())
	if secondsLeft < 0 {
		secondsLeft = 0
	}
	cur := s.coinCounter.Current()
	var pulses uint64
	if cur >= sess.BaseCount {
		pulses = cur - sess.BaseCount
	}
	amount := int64(pulses) * int64(s.coinPesoPerPulse)
	rates := s.getRates(r.Context())
	mins, usedAmount := convertAmountToMinutes(rates, amount)

	writeJSON(w, struct {
		Ok          bool   `json:"ok"`
		SecondsLeft int64  `json:"seconds_left"`
		Pulses      uint64 `json:"pulses"`
		Amount      int64  `json:"amount"`
		Minutes     int    `json:"minutes"`
		UsedAmount  int64  `json:"used_amount"`
	}{
		Ok:          true,
		SecondsLeft: secondsLeft,
		Pulses:      pulses,
		Amount:      amount,
		Minutes:     mins,
		UsedAmount:  usedAmount,
	})
}

func (s *Server) handleCoinCancel(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		var body struct {
			Token string `json:"token"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		token = strings.TrimSpace(body.Token)
	}
	if token == "" {
		http.Error(w, "missing token", http.StatusBadRequest)
		return
	}
	s.coinMu.Lock()
	if s.coinSess != nil && s.coinSess.Token == token {
		s.coinSess = nil
	}
	s.coinMu.Unlock()
	writeJSON(w, struct {
		Ok bool `json:"ok"`
	}{Ok: true})
}

func (s *Server) handleCoinCommit(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token string `json:"token"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	token := strings.TrimSpace(body.Token)
	if token == "" {
		token = strings.TrimSpace(r.URL.Query().Get("token"))
	}
	if token == "" {
		http.Error(w, "missing token", http.StatusBadRequest)
		return
	}

	now := time.Now().UTC()

	s.coinMu.Lock()
	sess := s.coinSess
	if sess == nil || sess.Token != token {
		s.coinMu.Unlock()
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if now.After(sess.ExpiresAt) {
		s.coinSess = nil
		s.coinMu.Unlock()
		http.Error(w, "expired", http.StatusBadRequest)
		return
	}
	s.coinSess = nil
	s.coinMu.Unlock()

	cur := s.coinCounter.Current()
	var pulses uint64
	if cur >= sess.BaseCount {
		pulses = cur - sess.BaseCount
	}
	amount := int64(pulses) * int64(s.coinPesoPerPulse)
	rates := s.getRates(r.Context())
	mins, usedAmount := convertAmountToMinutes(rates, amount)
	if mins <= 0 {
		http.Error(w, "no credit", http.StatusBadRequest)
		return
	}

	res, code, err := s.createAndConsumeMinutes(r.Context(), mins, sess.ClientIP)
	if err != nil {
		http.Error(w, "commit failed", http.StatusInternalServerError)
		return
	}

	writeJSON(w, struct {
		Ok         bool      `json:"ok"`
		Minutes    int       `json:"minutes"`
		Amount     int64     `json:"amount"`
		UsedAmount int64     `json:"used_amount"`
		Voucher    string    `json:"voucher"`
		EndAtUTC   time.Time `json:"end_at_utc"`
		ClientIP   string    `json:"client_ip"`
	}{
		Ok:         true,
		Minutes:    mins,
		Amount:     amount,
		UsedAmount: usedAmount,
		Voucher:    code,
		EndAtUTC:   res.Session.EndAt.UTC(),
		ClientIP:   sess.ClientIP,
	})
}

func (s *Server) getRates(ctx context.Context) []rate {
	val, ok, err := s.store.GetSetting(ctx, "rates")
	if err != nil {
		ok = false
	}
	if !ok || strings.TrimSpace(val) == "" {
		val = `[{"minutes":60,"price":10},{"minutes":180,"price":25},{"minutes":1440,"price":60}]`
	}
	var arr []rate
	if err := json.Unmarshal([]byte(val), &arr); err != nil {
		arr = nil
	}
	out := make([]rate, 0, len(arr))
	for _, r := range arr {
		if r.Minutes <= 0 || r.Price <= 0 {
			continue
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Price < out[j].Price
	})
	return out
}

func convertAmountToMinutes(rates []rate, amount int64) (int, int64) {
	if amount <= 0 || len(rates) == 0 {
		return 0, 0
	}
	type r2 struct {
		minutes int
		cents   int64
	}
	tmp := make([]r2, 0, len(rates))
	for _, r := range rates {
		c := int64(r.Price*100 + 0.5)
		if c <= 0 || r.Minutes <= 0 {
			continue
		}
		tmp = append(tmp, r2{minutes: r.Minutes, cents: c})
	}
	if len(tmp) == 0 {
		return 0, 0
	}
	sort.Slice(tmp, func(i, j int) bool { return tmp[i].cents > tmp[j].cents })

	remaining := amount * 100
	used := int64(0)
	minutes := 0

	for _, r := range tmp {
		if remaining < r.cents {
			continue
		}
		n := remaining / r.cents
		if n <= 0 {
			continue
		}
		remaining -= n * r.cents
		used += n * r.cents
		minutes += int(n) * r.minutes
	}
	return minutes, used / 100
}

func (s *Server) createAndConsumeMinutes(ctx context.Context, minutes int, ip string) (store.ConsumeVoucherResult, string, error) {
	for i := 0; i < 5; i++ {
		code, err := newVoucherCode(8)
		if err != nil {
			return store.ConsumeVoucherResult{}, "", err
		}
		if _, err := s.store.CreateVoucher(ctx, code, minutes); err != nil {
			continue
		}
		res, err := s.consumeVoucher(ctx, code, "", ip)
		if err != nil {
			return store.ConsumeVoucherResult{}, "", err
		}
		return res, code, nil
	}
	return store.ConsumeVoucherResult{}, "", errors.New("could not create voucher")
}

func (s *Server) handleAdmin(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Has("logout") {
		w.Header().Set("WWW-Authenticate", `Basic realm="admin"`)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if !s.requireAdmin(w, r) {
		return
	}
	page := strings.TrimSpace(r.URL.Query().Get("page"))
	if page == "" {
		page = "dashboard"
	}
	data := struct {
		Title string
		Page  string
	}{
		Title: s.title,
		Page:  page,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = s.adminTmpl.Execute(w, data)
}

func (s *Server) handleAdminSummary(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	now := time.Now().UTC()
	active, _ := s.store.CountActiveSessions(r.Context(), now)
	vcount, _ := s.store.CountVouchers(r.Context())
	board := gpio.Detect()
	writeJSON(w, struct {
		ActiveSessions int64  `json:"active_sessions"`
		Vouchers       int64  `json:"vouchers"`
		GatewayIP      string `json:"gateway_ip"`
		TimeUTC        string `json:"time_utc"`
		BoardModel     string `json:"board_model"`
		GPIO           any    `json:"gpio"`
	}{
		ActiveSessions: active,
		Vouchers:       vcount,
		GatewayIP:      r.Host,
		TimeUTC:        now.Format(time.RFC3339),
		BoardModel:     board.Model,
		GPIO: struct {
			Disabled bool   `json:"disabled"`
			CoinPin  int    `json:"coin_pin"`
			BillPin  int    `json:"bill_pin"`
			RelayPin int    `json:"relay_pin"`
			CoinEdge string `json:"coin_edge"`
			BillEdge string `json:"bill_edge"`
			Relay    string `json:"relay_active"`
		}{
			Disabled: board.Config.GPIODisabled,
			CoinPin:  board.Config.CoinPin,
			BillPin:  board.Config.BillPin,
			RelayPin: board.Config.RelayPin,
			CoinEdge: board.Config.CoinPinEdge,
			BillEdge: board.Config.BillPinEdge,
			Relay:    board.Config.RelayPinActive,
		},
	})
}

func (s *Server) handleAdminInterfaces(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	type iface struct {
		Name         string   `json:"name"`
		Index        int      `json:"index"`
		MTU          int      `json:"mtu"`
		HardwareAddr string   `json:"hardware_addr,omitempty"`
		Flags        []string `json:"flags"`
		Addrs        []string `json:"addrs"`
	}
	nifs, err := net.Interfaces()
	if err != nil {
		http.Error(w, "interfaces failed", http.StatusInternalServerError)
		return
	}
	out := make([]iface, 0, len(nifs))
	for _, ni := range nifs {
		addrs, _ := ni.Addrs()
		addrStrings := make([]string, 0, len(addrs))
		for _, a := range addrs {
			addrStrings = append(addrStrings, a.String())
		}
		flags := strings.Fields(ni.Flags.String())
		out = append(out, iface{
			Name:         ni.Name,
			Index:        ni.Index,
			MTU:          ni.MTU,
			HardwareAddr: ni.HardwareAddr.String(),
			Flags:        flags,
			Addrs:        addrStrings,
		})
	}
	defIface, defGW := readDefaultRoute()
	writeJSON(w, struct {
		DefaultInterface string  `json:"default_interface,omitempty"`
		DefaultGateway   string  `json:"default_gateway,omitempty"`
		Interfaces       []iface `json:"interfaces"`
	}{
		DefaultInterface: defIface,
		DefaultGateway:   defGW,
		Interfaces:       out,
	})
}

func (s *Server) handleAdminVouchers(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	limit := 100
	if v := strings.TrimSpace(r.URL.Query().Get("limit")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}
	vouchers, err := s.store.ListVouchers(r.Context(), limit)
	if err != nil {
		http.Error(w, "vouchers failed", http.StatusInternalServerError)
		return
	}
	type voucherRow struct {
		Code         string     `json:"code"`
		Minutes      int        `json:"minutes"`
		CreatedAtUTC time.Time  `json:"created_at_utc"`
		UsedAtUTC    *time.Time `json:"used_at_utc,omitempty"`
		UsedByIP     string     `json:"used_by_ip,omitempty"`
		UsedByMAC    string     `json:"used_by_mac,omitempty"`
	}
	rows := make([]voucherRow, 0, len(vouchers))
	for _, v := range vouchers {
		row := voucherRow{
			Code:         v.Code,
			Minutes:      v.Minutes,
			CreatedAtUTC: v.CreatedAt.UTC(),
			UsedByIP:     v.UsedByIP.String,
			UsedByMAC:    v.UsedByMAC.String,
		}
		if v.UsedAt.Valid {
			t := v.UsedAt.Time.UTC()
			row.UsedAtUTC = &t
		}
		rows = append(rows, row)
	}
	writeJSON(w, struct {
		Items []voucherRow `json:"items"`
	}{
		Items: rows,
	})
}

func (s *Server) handleAdminLogs(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	limit := 100
	if v := strings.TrimSpace(r.URL.Query().Get("limit")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}
	vouchers, _ := s.store.ListVouchers(r.Context(), limit)
	sessions, _ := s.store.ListSessions(r.Context(), limit)
	type event struct {
		TimeUTC  time.Time `json:"time_utc"`
		Type     string    `json:"type"`
		Message  string    `json:"message"`
		ClientIP string    `json:"client_ip,omitempty"`
	}
	evs := make([]event, 0, len(vouchers)+len(sessions))
	for _, v := range vouchers {
		evs = append(evs, event{
			TimeUTC: v.CreatedAt.UTC(),
			Type:    "voucher_created",
			Message: "Voucher created: " + v.Code,
		})
		if v.UsedAt.Valid {
			ip := strings.TrimSpace(v.UsedByIP.String)
			evs = append(evs, event{
				TimeUTC:  v.UsedAt.Time.UTC(),
				Type:     "voucher_used",
				Message:  "Voucher used: " + v.Code,
				ClientIP: ip,
			})
		}
	}
	for _, sess := range sessions {
		ip := strings.TrimSpace(sess.IP)
		evs = append(evs, event{
			TimeUTC:  sess.StartAt.UTC(),
			Type:     "session_start",
			Message:  "Session started",
			ClientIP: ip,
		})
	}
	sort.Slice(evs, func(i, j int) bool {
		return evs[i].TimeUTC.After(evs[j].TimeUTC)
	})
	if len(evs) > limit {
		evs = evs[:limit]
	}
	writeJSON(w, struct {
		Items []event `json:"items"`
	}{
		Items: evs,
	})
}

func (s *Server) handleAdminGetRates(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	const key = "rates"
	val, ok, err := s.store.GetSetting(r.Context(), key)
	if err != nil {
		http.Error(w, "get rates failed", http.StatusInternalServerError)
		return
	}
	if !ok || strings.TrimSpace(val) == "" {
		val = `[{"minutes":60,"price":10},{"minutes":180,"price":25},{"minutes":1440,"price":60}]`
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(val))
}

func (s *Server) handleAdminSetRates(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	defer r.Body.Close()
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 1024)
	for {
		n, err := r.Body.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			break
		}
	}
	js := strings.TrimSpace(string(buf))
	if js == "" {
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}
	if err := s.store.SetSetting(r.Context(), "rates", js); err != nil {
		http.Error(w, "set rates failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, struct {
		Ok bool `json:"ok"`
	}{Ok: true})
}

func (s *Server) handleAdminSubVendoDevices(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	now := time.Now().UTC()
	_, gw := readDefaultRoute()
	board := gpio.Detect()

	type device struct {
		Name          string `json:"name"`
		Mac           string `json:"mac"`
		ID            string `json:"id"`
		License       string `json:"license"`
		Status        string `json:"status"`
		Interface     string `json:"interface"`
		Version       string `json:"version"`
		LastActiveUTC string `json:"last_active_utc"`
		Info          string `json:"info"`
	}

	items := []device{
		{
			Name:          "Main Vendo",
			Mac:           "",
			ID:            "MAIN",
			License:       "ACTIVE",
			Status:        "Online Now",
			Interface:     "br10",
			Version:       "v1",
			LastActiveUTC: now.Format(time.RFC3339),
			Info:          board.Model + " • GW " + gw,
		},
	}
	writeJSON(w, struct {
		Items []device `json:"items"`
	}{
		Items: items,
	})
}
func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	if s.adminUser == "" {
		return true
	}
	u, p, ok := r.BasicAuth()
	if !ok || u != s.adminUser || p != s.adminPass {
		w.Header().Set("WWW-Authenticate", `Basic realm="admin"`)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return false
	}
	return true
}

func readDefaultRoute() (string, string) {
	b, err := os.ReadFile("/proc/net/route")
	if err != nil {
		return "", ""
	}
	lines := strings.Split(string(b), "\n")
	for _, ln := range lines[1:] {
		fields := strings.Fields(ln)
		if len(fields) < 3 {
			continue
		}
		if fields[1] != "00000000" {
			continue
		}
		iface := fields[0]
		gwHex := fields[2]
		if len(gwHex) != 8 {
			return iface, ""
		}
		v, err := strconv.ParseUint(gwHex, 16, 32)
		if err != nil {
			return iface, ""
		}
		ip := net.IPv4(byte(v), byte(v>>8), byte(v>>16), byte(v>>24)).String()
		return iface, ip
	}
	return "", ""
}

func (s *Server) consumeVoucher(ctx context.Context, code string, mac string, ip string) (store.ConsumeVoucherResult, error) {
	if code == "" {
		return store.ConsumeVoucherResult{}, errors.New("empty voucher code")
	}
	res, err := s.store.ConsumeVoucher(ctx, store.ConsumeVoucherParams{
		Code: code,
		MAC:  mac,
		IP:   ip,
		Now:  time.Now().UTC(),
	})
	if err != nil {
		return store.ConsumeVoucherResult{}, err
	}
	if err := s.allowlister.AllowIP4(ctx, ip, res.Session.EndAt); err != nil {
		return store.ConsumeVoucherResult{}, err
	}
	return res, nil
}

func clientIPFromRequest(r *http.Request) string {
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			ip := strings.TrimSpace(parts[0])
			if parsed := net.ParseIP(ip); parsed != nil {
				return parsed.String()
			}
		}
	}
	if xr := strings.TrimSpace(r.Header.Get("X-Real-IP")); xr != "" {
		if parsed := net.ParseIP(xr); parsed != nil {
			return parsed.String()
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	if parsed := net.ParseIP(r.RemoteAddr); parsed != nil {
		return parsed.String()
	}
	return strings.TrimSpace(r.RemoteAddr)
}

func newVoucherCode(length int) (string, error) {
	if length <= 0 {
		length = 8
	}
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	s := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf)
	s = strings.TrimRight(s, "=")
	s = strings.ToUpper(s)
	s = strings.ReplaceAll(s, "O", "8")
	s = strings.ReplaceAll(s, "I", "9")
	return s[:length], nil
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}
