// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build e2e

package api_test

import (
	"context"

	"connectrpc.com/connect"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/proto"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
)

var _ = Describe("schema validation", Ordered, func() {
	var (
		ctx  context.Context
		pool *pgxpool.Pool
	)

	BeforeAll(func() {
		ctx = context.Background()
		var err error
		pool, err = pgxpool.New(ctx, pgConnURL)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(pool.Close)
	})

	Describe("structural validation", func() {
		It("all 12 tables exist", func() {
			expected := []string{
				"projects",
				"specs",
				"decisions",
				"slices",
				"edges",
				"changelog_entries",
				"findings",
				"conversation_logs",
				"claims",
				"execution_events",
				"constitutions",
				"sync_mappings",
			}

			rows, err := pool.Query(ctx, `
				SELECT table_name
				FROM information_schema.tables
				WHERE table_schema = 'public'
				  AND table_type = 'BASE TABLE'
				  AND table_name = ANY($1)
			`, expected)
			Expect(err).NotTo(HaveOccurred())
			defer rows.Close()

			var found []string
			for rows.Next() {
				var name string
				Expect(rows.Scan(&name)).To(Succeed())
				found = append(found, name)
			}
			Expect(rows.Err()).NotTo(HaveOccurred())
			Expect(found).To(ConsistOf(expected))
		})

		It("specs.embedding column is USER-DEFINED (vector type)", func() {
			var dataType string
			err := pool.QueryRow(ctx, `
				SELECT data_type
				FROM information_schema.columns
				WHERE table_schema = 'public'
				  AND table_name   = 'specs'
				  AND column_name  = 'embedding'
			`).Scan(&dataType)
			Expect(err).NotTo(HaveOccurred())
			Expect(dataType).To(Equal("USER-DEFINED"))
		})

		It("specs.stage column is text", func() {
			var dataType string
			err := pool.QueryRow(ctx, `
				SELECT data_type
				FROM information_schema.columns
				WHERE table_schema = 'public'
				  AND table_name   = 'specs'
				  AND column_name  = 'stage'
			`).Scan(&dataType)
			Expect(err).NotTo(HaveOccurred())
			Expect(dataType).To(Equal("text"))
		})

		It("decisions.tags column is ARRAY", func() {
			var dataType string
			err := pool.QueryRow(ctx, `
				SELECT data_type
				FROM information_schema.columns
				WHERE table_schema = 'public'
				  AND table_name   = 'decisions'
				  AND column_name  = 'tags'
			`).Scan(&dataType)
			Expect(err).NotTo(HaveOccurred())
			Expect(dataType).To(Equal("ARRAY"))
		})

		It("edges.content_hash_at_link column is text", func() {
			var dataType string
			err := pool.QueryRow(ctx, `
				SELECT data_type
				FROM information_schema.columns
				WHERE table_schema = 'public'
				  AND table_name   = 'edges'
				  AND column_name  = 'content_hash_at_link'
			`).Scan(&dataType)
			Expect(err).NotTo(HaveOccurred())
			Expect(dataType).To(Equal("text"))
		})

		It("pgvector extension is installed", func() {
			var extname string
			err := pool.QueryRow(ctx, `
				SELECT extname
				FROM pg_extension
				WHERE extname = 'vector'
			`).Scan(&extname)
			Expect(err).NotTo(HaveOccurred())
			Expect(extname).To(Equal("vector"))
		})

		It("key indexes exist", func() {
			expected := []string{
				"idx_edges_forward",
				"idx_edges_reverse",
				"idx_changelog_spec",
				"idx_findings_spec",
			}

			rows, err := pool.Query(ctx, `
				SELECT indexname
				FROM pg_indexes
				WHERE schemaname = 'public'
				  AND indexname = ANY($1)
			`, expected)
			Expect(err).NotTo(HaveOccurred())
			defer rows.Close()

			var found []string
			for rows.Next() {
				var name string
				Expect(rows.Scan(&name)).To(Succeed())
				found = append(found, name)
			}
			Expect(rows.Err()).NotTo(HaveOccurred())
			Expect(found).To(ConsistOf(expected))
		})

		It("FK constraint exists: specs references projects", func() {
			var count int
			err := pool.QueryRow(ctx, `
				SELECT count(*)
				FROM information_schema.table_constraints tc
				JOIN information_schema.referential_constraints rc
				  ON rc.constraint_name = tc.constraint_name
				  AND rc.constraint_schema = tc.constraint_schema
				JOIN information_schema.constraint_column_usage ccu
				  ON ccu.constraint_name = rc.unique_constraint_name
				  AND ccu.constraint_schema = rc.unique_constraint_schema
				WHERE tc.constraint_type    = 'FOREIGN KEY'
				  AND tc.table_schema       = 'public'
				  AND tc.table_name         = 'specs'
				  AND ccu.table_name        = 'projects'
			`).Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(BeNumerically(">=", 1))
		})

		It("FK constraint exists: slices references specs", func() {
			var count int
			err := pool.QueryRow(ctx, `
				SELECT count(*)
				FROM information_schema.table_constraints tc
				JOIN information_schema.referential_constraints rc
				  ON rc.constraint_name = tc.constraint_name
				  AND rc.constraint_schema = tc.constraint_schema
				JOIN information_schema.constraint_column_usage ccu
				  ON ccu.constraint_name = rc.unique_constraint_name
				  AND ccu.constraint_schema = rc.unique_constraint_schema
				WHERE tc.constraint_type    = 'FOREIGN KEY'
				  AND tc.table_schema       = 'public'
				  AND tc.table_name         = 'slices'
				  AND ccu.table_name        = 'specs'
			`).Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(BeNumerically(">=", 1))
		})
	})

	Describe("behavioral validation", Ordered, func() {
		var specClient specgraphv1connect.SpecServiceClient

		BeforeAll(func() {
			specClient = newSpecClient()
		})

		It("create spec via API → changelog entry with version=1 exists in DB", func() {
			slug := "schema-val-changelog"

			_, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
				Slug:     slug,
				Intent:   "Schema validation changelog test",
				Priority: "p3",
			}))
			Expect(err).NotTo(HaveOccurred())

			var version int
			err = pool.QueryRow(ctx, `
				SELECT version
				FROM changelog_entries
				WHERE project_slug = $1
				  AND spec_slug    = $2
				ORDER BY version ASC
				LIMIT 1
			`, e2eProject, slug).Scan(&version)
			Expect(err).NotTo(HaveOccurred())
			Expect(version).To(Equal(1))
		})

		It("update spec via API → version incremented to 2 in DB", func() {
			slug := "schema-val-version"

			_, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
				Slug:     slug,
				Intent:   "Schema validation version test",
				Priority: "p3",
			}))
			Expect(err).NotTo(HaveOccurred())

			_, err = specClient.UpdateSpec(ctx, connect.NewRequest(&specv1.UpdateSpecRequest{
				Slug:   slug,
				Intent: proto.String("Schema validation version test updated"),
			}))
			Expect(err).NotTo(HaveOccurred())

			var version int
			err = pool.QueryRow(ctx, `
				SELECT version
				FROM specs
				WHERE project_slug = $1
				  AND slug = $2
			`, e2eProject, slug).Scan(&version)
			Expect(err).NotTo(HaveOccurred())
			Expect(version).To(Equal(2))
		})

		It("WipeProjectData clears specs and edges but preserves project row", func() {
			// Seed a spec and an edge.
			slugA := "schema-wipe-a"
			slugB := "schema-wipe-b"

			for _, slug := range []string{slugA, slugB} {
				_, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
					Slug:     slug,
					Intent:   "wipe test " + slug,
					Priority: "p3",
				}))
				Expect(err).NotTo(HaveOccurred())
			}

			graphClient := newGraphClient()
			_, err := graphClient.AddEdge(ctx, connect.NewRequest(&specv1.AddEdgeRequest{
				FromSlug: slugA,
				ToSlug:   slugB,
				EdgeType: specv1.EdgeType_EDGE_TYPE_DEPENDS_ON,
			}))
			Expect(err).NotTo(HaveOccurred())

			// Wipe via the store (scoped to e2e-test project).
			Expect(serverInfo.Store.WipeProjectData(ctx)).To(Succeed())

			var specCount, edgeCount, projectCount int

			err = pool.QueryRow(ctx, `
				SELECT count(*) FROM specs WHERE project_slug = $1
			`, e2eProject).Scan(&specCount)
			Expect(err).NotTo(HaveOccurred())
			Expect(specCount).To(Equal(0))

			err = pool.QueryRow(ctx, `
				SELECT count(*) FROM edges WHERE project_slug = $1
			`, e2eProject).Scan(&edgeCount)
			Expect(err).NotTo(HaveOccurred())
			Expect(edgeCount).To(Equal(0))

			err = pool.QueryRow(ctx, `
				SELECT count(*) FROM projects WHERE slug = $1
			`, e2eProject).Scan(&projectCount)
			Expect(err).NotTo(HaveOccurred())
			Expect(projectCount).To(Equal(1))
		})
	})
})
