package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/netip"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"linux_wifi/internal/config"
	"linux_wifi/internal/portal"
	"linux_wifi/internal/store"
)

func main() {
	log.SetFlags(0)

	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "serve":
		serve(os.Args[2:])
	case "render":
		render(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
}

func serve(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	listenAddr := fs.String("listen", ":8080", "HTTP listen address")
	dbPath := fs.String("db", "pisowifi.db", "SQLite DB path")
	portalTitle := fs.String("title", "PiSoWiFi", "Portal title")

	nftEnable := fs.Bool("nft-enable", false, "Enable nftables allowlisting (Linux only)")
	nftTable := fs.String("nft-table", "inet pisowifi", `nftables table, e.g. "inet pisowifi"`)
	nftAllowed4 := fs.String("nft-allowed4-set", "allowed4", "nftables set for allowed IPv4 addresses")
	nftExecPath := fs.String("nft-path", "nft", "Path to nft binary")
	allowMinutes := fs.Int("default-minutes", 60, "Default minutes if not provided")
	_ = fs.Parse(args)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	st, err := store.OpenSQLite(ctx, *dbPath)
	if err != nil {
		log.Fatalf("db open: %v", err)
	}
	defer st.Close()

	var allowlister portal.Allowlister = portal.NoopAllowlister{}
	if *nftEnable && runtime.GOOS == "linux" {
		allowlister = portal.NFTAllowlister{
			ExecPath:    *nftExecPath,
			Table:       *nftTable,
			Allowed4:    *nftAllowed4,
			TimeoutSkew: 5 * time.Second,
		}
	}

	srv := portal.NewServer(portal.ServerDeps{
		Store:          st,
		Allowlister:    allowlister,
		DefaultMinutes: *allowMinutes,
		Title:          *portalTitle,
	})
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	httpServer := &http.Server{
		Addr:              *listenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		stop := make(chan os.Signal, 2)
		signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
		<-stop
		cancel()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	fmt.Printf("listening on %s\n", *listenAddr)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("http: %v", err)
	}
}

func render(args []string) {
	if len(args) < 1 {
		renderUsage()
		os.Exit(2)
	}

	switch args[0] {
	case "nft":
		fs := flag.NewFlagSet("render nft", flag.ExitOnError)
		lanIf := fs.String("lan-if", "br0", "LAN interface (bridge)")
		wanIf := fs.String("wan-if", "eth0", "WAN/uplink interface")
		portalPort := fs.Int("portal-port", 8080, "Portal HTTP port")
		clientCIDR := fs.String("client-cidr", "10.10.0.0/16", "Client CIDR (IPv4)")
		table := fs.String("table", "pisowifi", "nftables table name")
		family := fs.String("family", "inet", "nftables family")
		allowedSet := fs.String("allowed4-set", "allowed4", "nftables allowed IPv4 set name")
		_ = fs.Parse(args[1:])

		prefix, err := netip.ParsePrefix(*clientCIDR)
		if err != nil {
			log.Fatalf("client-cidr: %v", err)
		}
		out, err := config.RenderNFTables(config.NFTablesConfig{
			TableFamily:  *family,
			TableName:    *table,
			Allowed4Set:  *allowedSet,
			WANInterface: *wanIf,
			LANInterface: *lanIf,
			PortalPort:   uint16(*portalPort),
			ClientCIDR:   prefix,
		})
		if err != nil {
			log.Fatal(err)
		}
		fmt.Print(out)
	case "dnsmasq":
		fs := flag.NewFlagSet("render dnsmasq", flag.ExitOnError)
		iface := fs.String("if", "br0.10", "Interface name")
		start := fs.String("start", "10.10.10.50", "DHCP range start (IPv4)")
		end := fs.String("end", "10.10.10.200", "DHCP range end (IPv4)")
		lease := fs.String("lease", "12h", "DHCP lease time (dnsmasq format)")
		router := fs.String("router", "", "Router IP (optional)")
		dns := fs.String("dns", "", "DNS IP (optional)")
		domain := fs.String("domain", "", "Domain (optional)")
		_ = fs.Parse(args[1:])

		startIP, err := netip.ParseAddr(*start)
		if err != nil {
			log.Fatalf("start: %v", err)
		}
		endIP, err := netip.ParseAddr(*end)
		if err != nil {
			log.Fatalf("end: %v", err)
		}
		var routerIP netip.Addr
		if strings.TrimSpace(*router) != "" {
			routerIP, err = netip.ParseAddr(*router)
			if err != nil {
				log.Fatalf("router: %v", err)
			}
		}
		var dnsIP netip.Addr
		if strings.TrimSpace(*dns) != "" {
			dnsIP, err = netip.ParseAddr(*dns)
			if err != nil {
				log.Fatalf("dns: %v", err)
			}
		}

		out, err := config.RenderDNSMasq(config.DNSMasqConfig{
			Ranges: []config.DHCPRange{
				{
					Interface: *iface,
					StartIP:   startIP,
					EndIP:     endIP,
					Lease:     *lease,
					RouterIP:  routerIP,
					DNSIP:     dnsIP,
					Domain:    *domain,
				},
			},
		})
		if err != nil {
			log.Fatal(err)
		}
		fmt.Print(out)
	case "hostapd":
		fs := flag.NewFlagSet("render hostapd", flag.ExitOnError)
		radio := fs.String("radio", "wlan0", "Radio interface")
		country := fs.String("country", "US", "Country code")
		channel := fs.Int("channel", 6, "Channel")
		ssid := fs.String("ssid", "PISO", "SSID name")
		bridge := fs.String("bridge", "br0.10", "Bridge interface for SSID")
		pass := fs.String("pass", "", "WPA2 password (optional)")
		_ = fs.Parse(args[1:])

		out, err := config.RenderHostapdMultiBSS(config.HostapdConfig{
			RadioInterface: *radio,
			CountryCode:    *country,
			Channel:        *channel,
			SSIDs: []config.SSIDConfig{
				{Name: *ssid, Bridge: *bridge, Password: *pass},
			},
		})
		if err != nil {
			log.Fatal(err)
		}
		fmt.Print(out)
	default:
		renderUsage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Println("pisowifi serve [flags]")
	fmt.Println("pisowifi render <nft|dnsmasq|hostapd> [flags]")
}

func renderUsage() {
	fmt.Println("pisowifi render nft --lan-if br0 --wan-if eth0 --portal-port 8080 --client-cidr 10.10.0.0/16")
	fmt.Println("pisowifi render dnsmasq --if br0.10 --start 10.10.10.50 --end 10.10.10.200 --lease 12h --router 10.10.10.1 --dns 10.10.10.1")
	fmt.Println("pisowifi render hostapd --radio wlan0 --ssid PISO --bridge br0.10 --pass \"password123\"")
}
