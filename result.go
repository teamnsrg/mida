package main

import (
	"github.com/chromedp/cdproto/debugger"
	"github.com/chromedp/cdproto/network"
)

type RawMIDAResult struct {
	SanitizedTask SanitizedMIDATask
	Stats         TaskStats
	Requests      map[string][]network.EventRequestWillBeSent
	Responses     map[string][]network.EventResponseReceived
	Scripts       map[string]*debugger.EventScriptParsed
}

type FinalMIDAResult struct {
	SanitizedTask SanitizedMIDATask
	Stats         TaskStats
}
