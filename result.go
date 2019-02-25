package main

import (
	"github.com/chromedp/cdproto/debugger"
	"github.com/chromedp/cdproto/network"
)

// The metadata for a single resource. May contain multiple requests
// and multiple responses, so they are each given as arrays. In general,
// they will usually (but not always) both have a length of 1.
type Resource struct {
	Requests  []network.EventRequestWillBeSent `json:"requests"`
	Responses []network.EventResponseReceived  `json:"responses"`
}

type RawMIDAResult struct {
	SanitizedTask SanitizedMIDATask
	Stats         TaskStats
	Requests      map[string][]network.EventRequestWillBeSent
	Responses     map[string][]network.EventResponseReceived
	Scripts       map[string]debugger.EventScriptParsed
}

type FinalMIDAResult struct {
	ResourceMetadata map[string]Resource
	SanitizedTask    SanitizedMIDATask
	ScriptMetadata   map[string]debugger.EventScriptParsed
	Stats            TaskStats
}
