package gpio

import (
	"os"
	"strings"
)

type Config struct {
	CoinPin         int
	RelayPin        int
	BillPin         int
	CoinPinEdge     string
	BillPinEdge     string
	RelayPinActive  string // "HIGH" or "LOW"
	GPIODisabled    bool
}

type BoardInfo struct {
	ModelRaw string
	Model    string
	Config   Config
}

// Detect tries to read the board model from common Linux locations
// and returns a best-effort GPIO mapping.
func Detect() BoardInfo {
	model := readFirstNonEmpty(
		"/sys/firmware/devicetree/base/model",
		"/proc/device-tree/model",
	)
	if model == "" {
		// fallback to cpuinfo hint
		model = cpuinfoModel()
	}
	model = strings.TrimSpace(model)
	canonical := canonicalModel(model)
	cfg := mappingForModel(canonical)
	return BoardInfo{
		ModelRaw: model,
		Model:    canonical,
		Config:   cfg,
	}
}

func readFirstNonEmpty(paths ...string) string {
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err == nil {
			s := strings.TrimSpace(string(b))
			if s != "" {
				return s
			}
		}
	}
	return ""
}

func cpuinfoModel() string {
	b, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return ""
	}
	lines := strings.Split(string(b), "\n")
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if strings.HasPrefix(strings.ToLower(ln), "model name") {
			parts := strings.SplitN(ln, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
		if strings.HasPrefix(strings.ToLower(ln), "hardware") {
			parts := strings.SplitN(ln, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return ""
}

func canonicalModel(raw string) string {
	r := strings.ToLower(raw)
	switch {
	case strings.Contains(r, "orange pi zero 3"):
		return "Orange Pi Zero 3"
	case strings.Contains(r, "orangepi zero3"):
		return "OrangePi Zero3"
	case strings.Contains(r, "orange pi one - op0100"):
		return "Orange Pi One - OP0100"
	case strings.Contains(r, "orange pi one"):
		return "Orange Pi One"
	case strings.Contains(r, "orange pi pc plus"):
		return "Orange Pi PC Plus"
	case strings.Contains(r, "orange pi pc - op0600"):
		return "Orange Pi PC - OP0600"
	case strings.Contains(r, "orange pi pc"):
		return "Orange Pi PC"
	case strings.Contains(r, "orange pi plus 2e"):
		return "Orange Pi Plus 2E"
	case strings.Contains(r, "orange pi zero"):
		return "Orange Pi Zero"
	case strings.Contains(r, "orange pi 4"):
		return "Orange Pi 4"
	case strings.Contains(r, "orange pi 3 - op0300"):
		return "Orange Pi 3 - OP0300"
	case strings.Contains(r, "orange pi 3"):
		return "Orange Pi 3"
	case strings.Contains(r, "raspberry pi 5"):
		return "Raspberry Pi 5"
	case strings.Contains(r, "raspberry pi 4"):
		return "Raspberry Pi 4B"
	case strings.Contains(r, "raspberry pi 3 model b+"):
		return "Raspberry Pi 3B+"
	case strings.Contains(r, "raspberry pi 3 model b"):
		return "Raspberry Pi 3B"
	case strings.Contains(r, "raspberry pi zero 2"):
		return "Raspberry Pi Zero 2 W"
	case strings.Contains(r, "raspberry pi zero"):
		return "Raspberry Pi Zero W"
	case strings.Contains(r, "nanopi neo2"):
		return "NanoPi NEO2"
	case strings.Contains(r, "nanopi neo"):
		return "NanoPi NEO"
	case strings.Contains(r, "nanopi m1"):
		return "NanoPi M1"
	default:
		// try generic hints
		if strings.Contains(r, "x86") || strings.Contains(r, "intel") || strings.Contains(r, "amd") {
			return "Generic x86_64"
		}
	}
	return "Unknown"
}

func mappingForModel(model string) Config {
	// Defaults for "Generic x86_64" and Unknown: disable GPIO
	def := Config{GPIODisabled: true}

	switch model {
	case "Orange Pi One", "Orange Pi One - OP0100",
		"Orange Pi PC", "Orange Pi PC - OP0600", "Orange Pi PC Plus",
		"Orange Pi Plus 2E", "Orange Pi Zero", "Orange Pi 3", "Orange Pi 3 - OP0300",
		"Orange Pi 4",
		"NanoPi NEO", "NanoPi NEO2", "NanoPi M1":
		return Config{
			CoinPin:        12, // PA12 (physical pin 3 in WiringOP numbering)
			RelayPin:       11, // PA11 (physical pin 5)
			BillPin:        6,  // PA6  (physical pin 7)
			CoinPinEdge:    "rising",
			BillPinEdge:    "falling",
			RelayPinActive: "HIGH",
		}
	case "Orange Pi Zero 3", "OrangePi Zero3":
		return Config{
			CoinPin:        229, // check board docs; typical mapping
			RelayPin:       228,
			BillPin:        72,  // PC9
			CoinPinEdge:    "rising",
			BillPinEdge:    "falling",
			RelayPinActive: "HIGH",
		}
	case "Raspberry Pi Zero W", "Raspberry Pi Zero 2 W",
		"Raspberry Pi 3B", "Raspberry Pi 3B+",
		"Raspberry Pi 4B", "Raspberry Pi 4 Model B",
		"Raspberry Pi 5":
		return Config{
			CoinPin:        2, // BCM2 (physical 3)
			RelayPin:       3, // BCM3 (physical 5)
			BillPin:        4, // BCM4 (physical 7)
			CoinPinEdge:    "rising",
			BillPinEdge:    "falling",
			RelayPinActive: "HIGH",
		}
	case "Generic x86_64":
		return def
	default:
		return def
	}
}

