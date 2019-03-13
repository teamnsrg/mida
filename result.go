package main

import (
	"github.com/chromedp/cdproto/debugger"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/pmurley/mida/jstrace"
)

// The metadata for a single resource. May contain multiple requests
// and multiple responses, so they are each given as arrays. In general,
// they will usually (but not always) both have a length of 1.
type Resource struct {
	Requests  []network.EventRequestWillBeSent `json:"requests"`
	Responses []network.EventResponseReceived  `json:"responses"`
}

type WSConnection struct {
	Url            string                                 `json:"url"`
	Initiator      *network.Initiator                     `json:"initiator"`
	FramesSent     []*network.EventWebSocketFrameSent     `json:"frames_sent"`
	FramesReceived []*network.EventWebSocketFrameReceived `json:"frames_received"`
	FrameErrors    []*network.EventWebSocketFrameError    `json:"frame_errors"`
	TSStart        string                                 `json:"ts_start"`
	TSSEnd         string                                 `json:"ts_end"`
}

type RawMIDAResult struct {
	SanitizedTask SanitizedMIDATask
	Stats         TaskStats
	Requests      map[string][]network.EventRequestWillBeSent
	Responses     map[string][]network.EventResponseReceived
	Scripts       map[string]debugger.EventScriptParsed
	FrameTree     *page.FrameTree
	WebsocketData map[string]*WSConnection
}

type FinalMIDAResult struct {
	ResourceMetadata map[string]Resource
	SanitizedTask    SanitizedMIDATask
	ScriptMetadata   map[string]debugger.EventScriptParsed
	Stats            TaskStats
	JSTrace          *jstrace.JSTrace
	WebsocketData    map[string]*WSConnection
}
