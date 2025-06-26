package main

import (
	"github.com/cooldogedev/spectrum"
	"github.com/cooldogedev/spectrum/api"
	"github.com/cooldogedev/spectrum/server"
	"github.com/cooldogedev/spectrum/session/animation"
	"github.com/cooldogedev/spectrum/util"
	"github.com/josscoder/bedrockpack/pack"
	"github.com/pelletier/go-toml/v2"
	"github.com/sandertv/gophertunnel/minecraft"
	"log"
	"log/slog"
	"os"
	"time"
)

type Config struct {
	Discovery struct {
		Server         string `toml:"server"`
		FallbackServer string `toml:"fallbackServer"`
	} `toml:"discovery"`

	StatusProvider struct {
		ServerName    string `toml:"serverName"`
		ServerSubName string `toml:"serverSubName"`
	} `toml:"status_provider"`

	OTF struct {
		OrgName        string `toml:"orgName"`
		RepoName       string `toml:"repoName"`
		Branch         string `toml:"branch"`
		PAT            string `toml:"pat"`
		UpdateInterval string `toml:"updateInterval"`
	} `toml:"otf"`

	API struct {
		Listen string `toml:"listen"`
	} `toml:"api"`
}

func main() {
	logger := slog.Default()

	data, err := os.ReadFile("config.toml")
	if os.IsNotExist(err) {
		logger.Warn("config.toml not found, generating default config.toml")

		defaultCfg := defaultConfig()
		buf, err := toml.Marshal(defaultCfg)
		if err != nil {
			log.Fatalf("failed to marshal default config: %v", err)
		}

		if err := os.WriteFile("config.toml", buf, 0644); err != nil {
			log.Fatalf("failed to write default config.toml: %v", err)
		}

		data = buf
	} else if err != nil {
		log.Fatalf("failed to read config.toml: %v", err)
	}

	var config Config
	if err := toml.Unmarshal(data, &config); err != nil {
		log.Fatalf("failed to parse config.toml: %v", err)
	}

	opts := util.DefaultOpts()
	opts.ShutdownMessage = "Server is shutting down"

	proxy := spectrum.NewSpectrum(
		server.NewStaticDiscovery(config.Discovery.Server, config.Discovery.FallbackServer),
		logger,
		opts,
		nil,
	)

	listenConfig := minecraft.ListenConfig{
		StatusProvider:       util.NewStatusProvider(config.StatusProvider.ServerName, config.StatusProvider.ServerSubName),
		TexturePacksRequired: true,
	}

	interval, err := time.ParseDuration(config.OTF.UpdateInterval)
	if err != nil {
		log.Fatalf("invalid updateInterval format: %v", err)
	}

	otf := pack.OTFConfig{
		OrgName:        config.OTF.OrgName,
		RepoName:       config.OTF.RepoName,
		Branch:         config.OTF.Branch,
		PAT:            getPAT(config.OTF.PAT),
		UpdateInterval: interval,
	}.New(logger)

	if err := otf.Start(); err != nil {
		logger.Error("failed to start otf resource pack", "error", err)
		return
	}

	if err := proxy.Listen(listenConfig); err != nil {
		logger.Error("Failed to start spectrum proxy", "err", err)
		return
	}

	listener := proxy.Listener()
	otf.SetListener(listener)

	go func() {
		a := api.NewAPI(proxy.Registry(), logger, nil)
		if err := a.Listen(config.API.Listen); err != nil {
			logger.Error("Failed to start spectrum API", "err", err)
			return
		}

		for {
			_ = a.Accept()
		}
	}()

	for {
		s, err := proxy.Accept()
		if err != nil {
			continue
		}
		s.SetAnimation(&animation.Dimension{})
	}
}

func getPAT(pat string) string {
	if pat != "" {
		return pat
	}
	return os.Getenv("GITHUB_PAT")
}

func defaultConfig() Config {
	return Config{
		Discovery: struct {
			Server         string `toml:"server"`
			FallbackServer string `toml:"fallbackServer"`
		}{
			Server:         "127.0.0.1:19133",
			FallbackServer: "127.0.0.1:19133",
		},
		StatusProvider: struct {
			ServerName    string `toml:"serverName"`
			ServerSubName string `toml:"serverSubName"`
		}{
			ServerName:    "Spectrum Proxy",
			ServerSubName: "Default SubName",
		},
		OTF: struct {
			OrgName        string `toml:"orgName"`
			RepoName       string `toml:"repoName"`
			Branch         string `toml:"branch"`
			PAT            string `toml:"pat"`
			UpdateInterval string `toml:"updateInterval"`
		}{
			OrgName:        "your-org",
			RepoName:       "your-repo",
			Branch:         "main",
			PAT:            "",
			UpdateInterval: "10m",
		},
		API: struct {
			Listen string `toml:"listen"`
		}{
			Listen: ":19132",
		},
	}
}
