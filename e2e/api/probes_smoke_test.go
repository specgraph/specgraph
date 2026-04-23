// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build e2e

package api_test

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/specgraph/specgraph/internal/server/probes"
	"github.com/specgraph/specgraph/internal/storage/postgres"
)

var _ = Describe("Probes (smoke)", Label("probes"), func() {
	It("serves /livez and /readyz against a real Postgres pool", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		store, err := postgres.New(ctx, pgConnURL, postgres.WithProject("probes-smoke"))
		Expect(err).NotTo(HaveOccurred())
		storeClosed := false
		defer func() {
			if storeClosed {
				return
			}
			shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutCancel()
			_ = store.Close(shutCtx)
		}()

		ln, err := net.Listen("tcp", "127.0.0.1:0")
		Expect(err).NotTo(HaveOccurred())
		addr := ln.Addr().String()

		h := probes.New(ctx, store, 100*time.Millisecond, 500*time.Millisecond)
		srv := &http.Server{
			Addr:              addr,
			Handler:           h.Mux(),
			ReadHeaderTimeout: 5 * time.Second,
		}
		go func() {
			_ = srv.Serve(ln)
		}()
		defer func() {
			shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutCancel()
			_ = srv.Shutdown(shutCtx)
		}()

		base := "http://" + addr

		By("/livez returns 200 once the listener accepts")
		Eventually(func() int {
			r, getErr := http.Get(base + "/livez") //nolint:noctx // retried via Eventually
			if getErr != nil {
				return 0
			}
			_ = r.Body.Close()
			return r.StatusCode
		}, 5*time.Second, 25*time.Millisecond).Should(Equal(http.StatusOK))

		By("/readyz reaches 200 after the first successful probe")
		Eventually(func() int {
			r, getErr := http.Get(base + "/readyz") //nolint:noctx // retried via Eventually
			if getErr != nil {
				return 0
			}
			_ = r.Body.Close()
			return r.StatusCode
		}, 5*time.Second, 25*time.Millisecond).Should(Equal(http.StatusOK))

		By("/readyz flips to 503 with a reason body after the pool is closed")
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
		Expect(store.Close(shutCtx)).To(Succeed())
		shutCancel()
		storeClosed = true

		var lastBody string
		Eventually(func() bool {
			r, getErr := http.Get(base + "/readyz") //nolint:noctx // retried via Eventually
			if getErr != nil {
				return false
			}
			body, _ := io.ReadAll(r.Body)
			_ = r.Body.Close()
			lastBody = string(body)
			return r.StatusCode == http.StatusServiceUnavailable &&
				strings.Contains(lastBody, "not ready:") &&
				strings.Contains(lastBody, "postgres")
		}, 5*time.Second, 25*time.Millisecond).Should(BeTrue(),
			"readyz must report 503 with a reason body identifying the postgres failure; last body: %q", lastBody)
	})
})
