// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/internal/config"
	"github.com/seanb4t/specgraph/internal/docker"
	"github.com/seanb4t/specgraph/internal/server"
	"github.com/seanb4t/specgraph/internal/storage"
	"github.com/seanb4t/specgraph/internal/storage/memgraph"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the SpecGraph server",
	RunE:  runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)
}

func runServe(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if cfg.IsRemote() {
		return fmt.Errorf("config has remote server set — no need to run serve")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if cfg.Server.Mode == "docker" {
		composeFile, err := docker.EnsureComposeFile(".", cfg.Storage.Backend)
		if err != nil {
			return err
		}
		fmt.Println("Starting Docker Compose stack...")
		if err := docker.ComposeUp(composeFile); err != nil {
			return err
		}
		defer func() {
			if err := docker.ComposeDown(composeFile); err != nil {
				fmt.Fprintf(os.Stderr, "warning: compose down: %v\n", err)
			}
		}()
	}

	switch cfg.Storage.Backend {
	case "memgraph":
		store, err := memgraph.New(ctx, cfg.Storage.Memgraph.BoltURI)
		if err != nil {
			return fmt.Errorf("connect to memgraph: %w", err)
		}
		defer func() {
			if err := store.Close(ctx); err != nil {
				fmt.Fprintf(os.Stderr, "warning: close store: %v\n", err)
			}
		}()

		constitutionPath := filepath.Join(filepath.Dir(cfgFile), "constitution.yaml")
		if err := bootstrapConstitution(ctx, store, constitutionPath); err != nil {
			fmt.Fprintf(os.Stderr, "warning: constitution bootstrap: %v\n", err)
		}

		mux := server.NewMux(store)
		server.RegisterHealthService(mux)
		server.RegisterDecisionService(mux, store)
		server.RegisterGraphService(mux, store)
		server.RegisterClaimService(mux, store)
		server.RegisterConstitutionService(mux, store)
		addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
		srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}

		go func() {
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			<-sigCh
			fmt.Println("\nShutting down...")
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer shutdownCancel()
			if err := srv.Shutdown(shutdownCtx); err != nil {
				fmt.Fprintf(os.Stderr, "warning: server shutdown: %v\n", err)
			}
		}()

		fmt.Printf("SpecGraph server running at http://%s\n", addr)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			return err
		}
	default:
		return fmt.Errorf("unsupported backend: %s", cfg.Storage.Backend)
	}

	return nil
}

type constitutionYAML struct {
	Name  string `yaml:"name"`
	Layer string `yaml:"layer"`
	Tech  struct {
		Languages struct {
			Primary   string   `yaml:"primary"`
			Allowed   []string `yaml:"allowed"`
			Forbidden []string `yaml:"forbidden"`
		} `yaml:"languages"`
		Frameworks     map[string]string `yaml:"frameworks"`
		Infrastructure map[string]string `yaml:"infrastructure"`
	} `yaml:"tech"`
	Principles  []string `yaml:"principles"`
	Constraints []string `yaml:"constraints"`
}

func bootstrapConstitution(ctx context.Context, store *memgraph.Store, yamlPath string) error {
	_, err := store.GetConstitution(ctx)
	if err == nil {
		return nil // already exists
	}
	if !errors.Is(err, storage.ErrConstitutionNotFound) {
		return fmt.Errorf("check existing constitution: %w", err)
	}

	data, err := os.ReadFile(yamlPath)
	if err != nil {
		return nil //nolint:nilerr // no YAML file is expected and acceptable
	}

	var cy constitutionYAML
	if err := yaml.Unmarshal(data, &cy); err != nil {
		return fmt.Errorf("parse constitution YAML: %w", err)
	}

	layerKey := "CONSTITUTION_LAYER_" + strings.ToUpper(cy.Layer)
	layerVal, ok := specv1.ConstitutionLayer_value[layerKey]
	if !ok {
		layerVal = int32(specv1.ConstitutionLayer_CONSTITUTION_LAYER_UNSPECIFIED)
	}

	principles := make([]*specv1.Principle, 0, len(cy.Principles))
	for _, p := range cy.Principles {
		principles = append(principles, &specv1.Principle{Principle: p})
	}

	constitution := &specv1.Constitution{
		Name:        cy.Name,
		Layer:       specv1.ConstitutionLayer(layerVal),
		Principles:  principles,
		Constraints: cy.Constraints,
		Tech: &specv1.TechConfig{
			Languages: &specv1.LanguageConfig{
				Primary:   cy.Tech.Languages.Primary,
				Allowed:   cy.Tech.Languages.Allowed,
				Forbidden: cy.Tech.Languages.Forbidden,
			},
			Frameworks:     cy.Tech.Frameworks,
			Infrastructure: cy.Tech.Infrastructure,
		},
	}

	if _, err := store.UpdateConstitution(ctx, constitution); err != nil {
		return fmt.Errorf("seed constitution: %w", err)
	}

	fmt.Println("Bootstrapped constitution from", yamlPath)
	return nil
}
