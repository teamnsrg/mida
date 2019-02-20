package main

import (
	"github.com/chromedp/cdproto/debugger"
	"github.com/chromedp/cdproto/network"
	"time"
)

type TimingResult struct {
	BrowserOpen           time.Time
	DevtoolsConnect       time.Time
	ConnectionEstablished time.Time
	LoadEvent             time.Time
	DOMContentEvent       time.Time
	BrowserClose          time.Time
}

type RawMIDAResult struct {
	SanitizedTask SanitizedMIDATask
	Stats         TaskStats
	Timing        TimingResult
	Requests      map[string][]network.EventRequestWillBeSent
	Responses     map[string][]network.EventResponseReceived
	Scripts       map[string]*debugger.EventScriptParsed
}

type FinalMIDAResult struct {
	SanitizedTask SanitizedMIDATask
	Stats         TaskStats
	Timing        TimingResult
}
