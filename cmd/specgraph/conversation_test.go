// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- fake handlers ---

type fakeConvRecordHandler struct {
	specgraphv1connect.UnimplementedAuthoringServiceHandler
}

func (fakeConvRecordHandler) RecordConversation(_ context.Context, _ *connect.Request[specv1.RecordConversationRequest]) (*connect.Response[specv1.RecordConversationResponse], error) {
	return connect.NewResponse(&specv1.RecordConversationResponse{
		ConversationLog: &specv1.ConversationLog{
			Id:            "conv-123",
			Stage:         "spark",
			ExchangeCount: 1,
		},
	}), nil
}

type fakeConvRecordErrHandler struct {
	specgraphv1connect.UnimplementedAuthoringServiceHandler
}

func (fakeConvRecordErrHandler) RecordConversation(_ context.Context, _ *connect.Request[specv1.RecordConversationRequest]) (*connect.Response[specv1.RecordConversationResponse], error) {
	return nil, connect.NewError(connect.CodeInternal, errors.New("record failed"))
}

type fakeConvRecordNilLogHandler struct {
	specgraphv1connect.UnimplementedAuthoringServiceHandler
}

func (fakeConvRecordNilLogHandler) RecordConversation(_ context.Context, _ *connect.Request[specv1.RecordConversationRequest]) (*connect.Response[specv1.RecordConversationResponse], error) {
	return connect.NewResponse(&specv1.RecordConversationResponse{}), nil
}

type fakeConvListHandler struct {
	specgraphv1connect.UnimplementedAuthoringServiceHandler
}

func (fakeConvListHandler) ListConversations(_ context.Context, _ *connect.Request[specv1.ListConversationsRequest]) (*connect.Response[specv1.ListConversationsResponse], error) {
	return connect.NewResponse(&specv1.ListConversationsResponse{
		ConversationLogs: []*specv1.ConversationLog{
			{Id: "conv-1", Stage: "spark", ExchangeCount: 3},
		},
	}), nil
}

type fakeConvListEmptyHandler struct {
	specgraphv1connect.UnimplementedAuthoringServiceHandler
}

func (fakeConvListEmptyHandler) ListConversations(_ context.Context, _ *connect.Request[specv1.ListConversationsRequest]) (*connect.Response[specv1.ListConversationsResponse], error) {
	return connect.NewResponse(&specv1.ListConversationsResponse{}), nil
}

type fakeConvListErrHandler struct {
	specgraphv1connect.UnimplementedAuthoringServiceHandler
}

func (fakeConvListErrHandler) ListConversations(_ context.Context, _ *connect.Request[specv1.ListConversationsRequest]) (*connect.Response[specv1.ListConversationsResponse], error) {
	return nil, connect.NewError(connect.CodeInternal, errors.New("list failed"))
}

// --- helpers ---

func setConvRecordVars(t *testing.T, jsonFile string, jsonOut bool) {
	t.Helper()

	oldFile := convRecordJSONFile
	oldStage := convRecordStage
	oldAmend := convRecordIsAmend
	oldJSON := convRecordJSON

	convRecordJSONFile = jsonFile
	convRecordStage = "spark"
	convRecordIsAmend = false
	convRecordJSON = jsonOut

	t.Cleanup(func() {
		convRecordJSONFile = oldFile
		convRecordStage = oldStage
		convRecordIsAmend = oldAmend
		convRecordJSON = oldJSON
	})
}

func setConvListVars(t *testing.T, jsonOut bool) {
	t.Helper()

	oldStage := convListStage
	oldJSON := convListJSON

	convListStage = ""
	convListJSON = jsonOut

	t.Cleanup(func() {
		convListStage = oldStage
		convListJSON = oldJSON
	})
}

// --- record tests ---

func TestRunConversationRecord_HappyPath(t *testing.T) {
	startFakeAuthoringServer(t, fakeConvRecordHandler{})
	path := writeJSONFile(t, `{"exchanges":[{"role":"user","content":"hello","stage":"spark","sequence":1}]}`)
	setConvRecordVars(t, path, false)

	err := runConversationRecord(newCmdWithCtx(), []string{"my-spec"})
	require.NoError(t, err)
}

func TestRunConversationRecord_HappyPath_JSON(t *testing.T) {
	startFakeAuthoringServer(t, fakeConvRecordHandler{})
	path := writeJSONFile(t, `{"exchanges":[{"role":"user","content":"hello","stage":"spark","sequence":1}]}`)
	setConvRecordVars(t, path, true)

	cmd := newCmdWithCtx()
	err := runConversationRecord(cmd, []string{"my-spec"})
	require.NoError(t, err)
}

func TestRunConversationRecord_MissingFile(t *testing.T) {
	startFakeAuthoringServer(t, fakeConvRecordHandler{})
	setConvRecordVars(t, filepath.Join(t.TempDir(), "does-not-exist-conv.json"), false)

	err := runConversationRecord(newCmdWithCtx(), []string{"my-spec"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "conversation record")
}

func TestRunConversationRecord_InvalidJSON(t *testing.T) {
	startFakeAuthoringServer(t, fakeConvRecordHandler{})
	path := writeJSONFile(t, `{not valid json`)
	setConvRecordVars(t, path, false)

	err := runConversationRecord(newCmdWithCtx(), []string{"my-spec"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "conversation record")
}

func TestRunConversationRecord_RPCError(t *testing.T) {
	startFakeAuthoringServer(t, fakeConvRecordErrHandler{})
	path := writeJSONFile(t, `{"exchanges":[{"role":"user","content":"hello","stage":"spark","sequence":1}]}`)
	setConvRecordVars(t, path, false)

	err := runConversationRecord(newCmdWithCtx(), []string{"my-spec"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "conversation record")
}

func TestRunConversationRecord_ClientError(t *testing.T) {
	setMissingConfig(t)
	path := writeJSONFile(t, `{"exchanges":[]}`)
	setConvRecordVars(t, path, false)

	err := runConversationRecord(newCmdWithCtx(), []string{"my-spec"})
	require.Error(t, err)
}

func TestRunConversationRecord_NilLog(t *testing.T) {
	startFakeAuthoringServer(t, fakeConvRecordNilLogHandler{})
	path := writeJSONFile(t, `{"exchanges":[{"role":"user","content":"hello","stage":"spark","sequence":1}]}`)
	setConvRecordVars(t, path, false)

	err := runConversationRecord(newCmdWithCtx(), []string{"my-spec"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing conversation_log")
}

// --- list tests ---

func TestRunConversationList_HappyPath(t *testing.T) {
	startFakeAuthoringServer(t, fakeConvListHandler{})
	setConvListVars(t, false)

	err := runConversationList(newCmdWithCtx(), []string{"my-spec"})
	require.NoError(t, err)
}

func TestRunConversationList_HappyPath_JSON(t *testing.T) {
	startFakeAuthoringServer(t, fakeConvListHandler{})
	setConvListVars(t, true)

	cmd := newCmdWithCtx()
	err := runConversationList(cmd, []string{"my-spec"})
	require.NoError(t, err)
}

func TestRunConversationList_Empty(t *testing.T) {
	startFakeAuthoringServer(t, fakeConvListEmptyHandler{})
	setConvListVars(t, false)

	err := runConversationList(newCmdWithCtx(), []string{"my-spec"})
	require.NoError(t, err)
}

func TestRunConversationList_RPCError(t *testing.T) {
	startFakeAuthoringServer(t, fakeConvListErrHandler{})
	setConvListVars(t, false)

	err := runConversationList(newCmdWithCtx(), []string{"my-spec"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "conversation list")
}

func TestRunConversationList_ClientError(t *testing.T) {
	setMissingConfig(t)
	setConvListVars(t, false)

	err := runConversationList(newCmdWithCtx(), []string{"my-spec"})
	require.Error(t, err)
}

// --- cobra arg validation ---

func TestConversationRecordCmd_RequiresSlug(t *testing.T) {
	err := conversationRecordCmd.Args(conversationRecordCmd, []string{})
	require.Error(t, err)
}

func TestConversationListCmd_RequiresSlug(t *testing.T) {
	err := conversationListCmd.Args(conversationListCmd, []string{})
	require.Error(t, err)
}
