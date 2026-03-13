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
	"strings"
	"time"

	"linux_wifi/internal/gpio"
	"linux_wifi/internal/store"
)

type ServerDeps struct {
	Store          *store.Store
	Allowlister    Allowlister
	DefaultMinutes int
	Title          string
	AdminUser      string
	AdminPass      string
}

type Server struct {
	store          *store.Store
	allowlister    Allowlister
	defaultMinutes int
	title          string
	portalTmpl     *template.Template
	adminUser      string
	adminPass      string
	adminTmpl      *template.Template
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

	return &Server{
		store:          d.Store,
		allowlister:    allowlister,
		defaultMinutes: mins,
		title:          title,
		portalTmpl:     tmpl,
		adminUser:      strings.TrimSpace(d.AdminUser),
		adminPass:      strings.TrimSpace(d.AdminPass),
		adminTmpl:      admin,
	}
}

func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /", s.handlePortal)
	mux.HandleFunc("POST /login", s.handlePortalLogin)

	mux.HandleFunc("POST /api/v1/vouchers", s.handleCreateVoucher)
	mux.HandleFunc("POST /api/v1/login", s.handleAPILogin)
	mux.HandleFunc("GET /api/v1/status", s.handleStatus)

	mux.HandleFunc("GET /admin", s.handleAdmin)
	mux.HandleFunc("GET /api/admin/summary", s.handleAdminSummary)
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

func (s *Server) handleAdmin(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Has("logout") {
		w.Header().Set("WWW-Authenticate", `Basic realm="admin"`)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if s.adminUser != "" {
		u, p, ok := r.BasicAuth()
		if !ok || u != s.adminUser || p != s.adminPass {
			w.Header().Set("WWW-Authenticate", `Basic realm="admin"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
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
	if s.adminUser != "" {
		u, p, ok := r.BasicAuth()
		if !ok || u != s.adminUser || p != s.adminPass {
			w.Header().Set("WWW-Authenticate", `Basic realm="admin"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
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
