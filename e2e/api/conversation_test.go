// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build e2e

package api_test

import (
	"context"

	"connectrpc.com/connect"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
)

var _ = Describe("Conversation logs", Ordered, func() {
	var (
		authoringClient specgraphv1connect.AuthoringServiceClient
		specClient      specgraphv1connect.SpecServiceClient
		ctx             context.Context
	)

	BeforeAll(func() {
		httpClient := projectClientFor("conversation-e2e")
		authoringClient = specgraphv1connect.NewAuthoringServiceClient(httpClient, serverInfo.BaseURL)
		specClient = specgraphv1connect.NewSpecServiceClient(httpClient, serverInfo.BaseURL)
		ctx = context.Background()
	})

	It("RecordConversation creates a log and returns it", func() {
		const slug = "conv-e2e-record"

		// Create spec via Spark.
		_, err := authoringClient.Spark(ctx, connect.NewRequest(&specv1.SparkRequest{
			Slug: slug,
			Output: &specv1.SparkOutput{
				Seed:   "conversation recording test",
				Signal: "testing RecordConversation RPC",
			},
		}))
		Expect(err).NotTo(HaveOccurred())

		// Record a conversation with 2 exchanges.
		resp, err := authoringClient.RecordConversation(ctx, connect.NewRequest(&specv1.RecordConversationRequest{
			Slug:  slug,
			Stage: "spark",
			Exchanges: []*specv1.ConversationExchange{
				{Role: "probe", Content: "What problem does this solve?", Stage: "spark"},
				{Role: "response", Content: "It solves X by doing Y.", Stage: "spark"},
			},
		}))
		Expect(err).NotTo(HaveOccurred())

		log := resp.Msg.ConversationLog
		Expect(log).NotTo(BeNil())
		Expect(log.Id).NotTo(BeEmpty())
		Expect(log.Stage).To(Equal("spark"))
		Expect(log.ExchangeCount).To(BeEquivalentTo(2))
		Expect(log.Exchanges).To(HaveLen(2))
	})

	It("ListConversations returns logs in chain order", func() {
		const slug = "conv-e2e-list"

		// Create spec and spark it.
		_, err := authoringClient.Spark(ctx, connect.NewRequest(&specv1.SparkRequest{
			Slug: slug,
			Output: &specv1.SparkOutput{
				Seed:   "list conversations test",
				Signal: "testing ListConversations ordering",
			},
		}))
		Expect(err).NotTo(HaveOccurred())

		// Record spark conversation.
		_, err = authoringClient.RecordConversation(ctx, connect.NewRequest(&specv1.RecordConversationRequest{
			Slug:  slug,
			Stage: "spark",
			Exchanges: []*specv1.ConversationExchange{
				{Role: "probe", Content: "Spark probe", Stage: "spark"},
			},
		}))
		Expect(err).NotTo(HaveOccurred())

		// Advance to shape.
		_, err = authoringClient.Shape(ctx, connect.NewRequest(&specv1.ShapeRequest{
			Slug: slug,
			Output: &specv1.ShapeOutput{
				ScopeIn:        []string{"feature A"},
				ScopeOut:       []string{"feature B"},
				Approaches:     []*specv1.Approach{{Name: "a1", Description: "approach one"}},
				ChosenApproach: "a1",
				SuccessMust:    []string{"works"},
			},
			Posture: specv1.Posture_POSTURE_DRIVE,
		}))
		Expect(err).NotTo(HaveOccurred())

		// Record shape conversation.
		_, err = authoringClient.RecordConversation(ctx, connect.NewRequest(&specv1.RecordConversationRequest{
			Slug:  slug,
			Stage: "shape",
			Exchanges: []*specv1.ConversationExchange{
				{Role: "probe", Content: "Shape probe", Stage: "shape"},
			},
		}))
		Expect(err).NotTo(HaveOccurred())

		// List all conversations.
		listResp, err := authoringClient.ListConversations(ctx, connect.NewRequest(&specv1.ListConversationsRequest{
			Slug: slug,
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(listResp.Msg.ConversationLogs).To(HaveLen(2))
		Expect(listResp.Msg.ConversationLogs[0].Stage).To(Equal("spark"))
		Expect(listResp.Msg.ConversationLogs[1].Stage).To(Equal("shape"))
	})

	It("ListConversations filters by stage", func() {
		const slug = "conv-e2e-list" // reuse from previous test (Ordered suite)

		// Filter to spark only.
		listResp, err := authoringClient.ListConversations(ctx, connect.NewRequest(&specv1.ListConversationsRequest{
			Slug:  slug,
			Stage: "spark",
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(listResp.Msg.ConversationLogs).To(HaveLen(1))
		Expect(listResp.Msg.ConversationLogs[0].Stage).To(Equal("spark"))
	})

	It("GetSpec includes stage outputs after authoring", func() {
		const slug = "conv-e2e-stage-out"

		// Create spec via Spark with specific seed.
		_, err := authoringClient.Spark(ctx, connect.NewRequest(&specv1.SparkRequest{
			Slug: slug,
			Output: &specv1.SparkOutput{
				Seed:       "stage output visibility test",
				Signal:     "testing GetSpec stage output population",
				ScopeSniff: specv1.ScopeSniff_SCOPE_SNIFF_SMALL,
				KillTest:   "no kill",
			},
			Posture: specv1.Posture_POSTURE_DRIVE,
		}))
		Expect(err).NotTo(HaveOccurred())

		// GetSpec should include the spark output.
		getResp, err := specClient.GetSpec(ctx, connect.NewRequest(&specv1.GetSpecRequest{
			Slug: slug,
		}))
		Expect(err).NotTo(HaveOccurred())

		spec := getResp.Msg.Spec
		Expect(spec).NotTo(BeNil())
		Expect(spec.SparkOutput).NotTo(BeNil())
		Expect(spec.SparkOutput.Seed).To(Equal("stage output visibility test"))
		Expect(spec.SparkOutput.KillTest).To(Equal("no kill"))
	})

	It("GetSpec includes conversation logs", func() {
		const slug = "conv-e2e-getspec-logs"

		// Create spec.
		_, err := authoringClient.Spark(ctx, connect.NewRequest(&specv1.SparkRequest{
			Slug: slug,
			Output: &specv1.SparkOutput{
				Seed:   "getspec conversation test",
				Signal: "testing conversation logs in GetSpec",
			},
		}))
		Expect(err).NotTo(HaveOccurred())

		// Record a conversation.
		_, err = authoringClient.RecordConversation(ctx, connect.NewRequest(&specv1.RecordConversationRequest{
			Slug:  slug,
			Stage: "spark",
			Exchanges: []*specv1.ConversationExchange{
				{Role: "probe", Content: "Why this approach?", Stage: "spark"},
				{Role: "response", Content: "Because reasons.", Stage: "spark"},
			},
		}))
		Expect(err).NotTo(HaveOccurred())

		// GetSpec should include conversation logs.
		getResp, err := specClient.GetSpec(ctx, connect.NewRequest(&specv1.GetSpecRequest{
			Slug: slug,
		}))
		Expect(err).NotTo(HaveOccurred())

		spec := getResp.Msg.Spec
		Expect(spec).NotTo(BeNil())
		Expect(spec.ConversationLogs).To(HaveLen(1))
		Expect(spec.ConversationLogs[0].Stage).To(Equal("spark"))
		Expect(spec.ConversationLogs[0].ExchangeCount).To(BeEquivalentTo(2))
	})
})
