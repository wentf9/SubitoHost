package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/wentf9/subitohost/internal/api"
	"github.com/wentf9/subitohost/internal/config"
	"github.com/wentf9/subitohost/internal/engine"
	"github.com/wentf9/subitohost/internal/i18n"
	"github.com/wentf9/subitohost/internal/util"
)

var (
	cfgPath string
	baseURL string
)

func main() {
	root := &cobra.Command{
		Use:   "subitohost",
		Short: i18n.T("Lightweight MIDI router and SoundFont host for live performance"),
	}

	root.PersistentFlags().StringVar(&cfgPath, "config", "~/.config/subitohost/config.json", i18n.T("config file path"))
	root.PersistentFlags().StringVar(&baseURL, "url", "http://127.0.0.1:3301", i18n.T("engine API base URL"))

	root.AddCommand(startCmd())
	root.AddCommand(stopCmd())
	root.AddCommand(statusCmd())
	root.AddCommand(devicesCmd())
	root.AddCommand(connectCmd())
	root.AddCommand(setlistCmd())
	root.AddCommand(recordCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func startCmd() *cobra.Command {
	var setlistPath string
	cmd := &cobra.Command{
		Use:   "start",
		Short: i18n.T("Start the engine daemon"),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				log.Printf("using default config: %v", err)
				cfg = config.Default()
			}
			cfg.ExpandPaths()

			e := engine.New(cfg)

			// Check for recovery
			if cfg.Recovery.AutoResume {
				if state, err := engine.LoadRecovery(cfg.Recovery.StateFile); err == nil {
					log.Printf("recovering: setlist=%s index=%d", state.SetlistPath, state.CurrentIndex)
					setlistPath = state.SetlistPath
					if err := e.LoadSetlist(state.SetlistPath, state.CurrentIndex); err != nil {
						log.Printf("recovery failed: %v", err)
					}
				}
			}

			// Load setlist if specified and not already loaded from recovery
			if setlistPath != "" && e.State() == nil {
				if err := e.LoadSetlist(setlistPath, 0); err != nil {
					return fmt.Errorf("load setlist: %w", err)
				}
			}

			if err := e.Start(); err != nil {
				return err
			}

			// Auto-connect MIDI
			if cfg.MIDI.AutoConnect && cfg.MIDI.DeviceNamePattern != "" {
				if err := e.AutoConnectMIDI(); err != nil {
					log.Printf("MIDI auto-connect: %v (will retry)", err)
				}
			}

			srv := api.NewServer(e, cfg.ListenAddr)
			go func() {
				if err := srv.Start(); err != nil && err != http.ErrServerClosed {
					log.Fatalf("API server: %v", err)
				}
			}()

			// Wait for shutdown signal
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			<-sigCh

			log.Println("shutting down...")
			srv.Shutdown(context.Background())
			e.Stop()
			return nil
		},
	}
	cmd.Flags().StringVar(&setlistPath, "setlist", "", i18n.T("setlist file to load on start"))
	return cmd
}

func stopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: i18n.T("Stop the engine"),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(i18n.T("Use Ctrl+C or kill the daemon process to stop."))
			return nil
		},
	}
}

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: i18n.T("Query engine status"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return apiGet("/api/v1/status")
		},
	}
}

func devicesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "devices",
		Short: i18n.T("List MIDI devices"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return apiGet("/api/v1/midi/devices")
		},
	}
}

func connectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "connect [device_id]",
		Short: i18n.T("Connect a MIDI device"),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return apiPost("/api/v1/midi/connect", fmt.Sprintf(`{"device_id": %s}`, args[0]))
		},
	}
}

func setlistCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setlist",
		Short: i18n.T("Manage the setlist"),
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "load [file]",
		Short: i18n.T("Load a setlist file"),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return apiPut("/api/v1/setlist", fmt.Sprintf(`{"path": %q}`, args[0]))
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "next",
		Short: i18n.T("Advance to next profile"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return apiPost("/api/v1/setlist/next", "")
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "prev",
		Short: i18n.T("Go to previous profile"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return apiPost("/api/v1/setlist/prev", "")
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "goto [index]",
		Short: i18n.T("Jump to profile at index"),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return apiPost("/api/v1/setlist/goto", fmt.Sprintf(`{"index": %s}`, args[0]))
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "show",
		Short: i18n.T("Show current setlist and position"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return apiGet("/api/v1/setlist")
		},
	})

	return cmd
}

func recordCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "record",
		Short: i18n.T("Control audio and MIDI recording"),
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "start",
		Short: i18n.T("Start recording"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return apiPost("/api/v1/record/start", "")
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "stop",
		Short: i18n.T("Stop recording and start WAV rendering"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return apiPost("/api/v1/record/stop", "")
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: i18n.T("Show recording status"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return apiGet("/api/v1/record/status")
		},
	})
	return cmd
}

// --- HTTP client helpers ---

func apiGet(path string) error {
	resp, err := http.Get(baseURL + path)
	if err != nil {
		return fmt.Errorf("cannot reach engine at %s: %w", baseURL, err)
	}
	defer resp.Body.Close()
	return printResponse(resp)
}

func apiPost(path, body string) error {
	resp, err := http.Post(baseURL+path, "application/json", strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("cannot reach engine at %s: %w", baseURL, err)
	}
	defer resp.Body.Close()
	return printResponse(resp)
}

func apiPut(path, body string) error {
	req, err := http.NewRequest("PUT", baseURL+path, strings.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("cannot reach engine at %s: %w", baseURL, err)
	}
	defer resp.Body.Close()
	return printResponse(resp)
}

func printResponse(resp *http.Response) error {
	data, _ := io.ReadAll(resp.Body)
	var pretty json.RawMessage
	if json.Unmarshal(data, &pretty) == nil {
		formatted, _ := json.MarshalIndent(pretty, "", "  ")
		fmt.Println(string(formatted))
	} else {
		fmt.Println(string(data))
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}
	return nil
}

func loadConfig() (*config.Config, error) {
	path := util.ExpandHome(cfgPath)
	return config.Load(path)
}
