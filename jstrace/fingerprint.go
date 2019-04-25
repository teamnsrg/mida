package jstrace

import (
	"github.com/teamnsrg/mida/log"
	"strconv"
)

// OpenWPMCheckTraceForFingerprinting checks a given JavScript trace for fingerprinting
// based on our interpretation of the checks laid out in the following paper:
// http://randomwalker.info/publications/OpenWPM_1_million_site_tracking_measurement.pdf
func OpenWPMCheckTraceForFingerprinting(trace *JSTrace) error {
	for _, isolate := range trace.Isolates {
		for _, script := range isolate.Scripts {
			err := openWPMCheckScript(script)
			if err != nil {
				log.Log.Error(err)
			}

			if script.OpenWPM.Canvas {
				log.Log.Infof("Found canvas fingerprinting: %s", script.BaseUrl)
			}
			if script.OpenWPM.CanvasFont {
				log.Log.Infof("Found canvas font fingerprinting: %s", script.BaseUrl)
			}
			if script.OpenWPM.WebRTC {
				log.Log.Infof("Found WebRTC fingerprinting: %s", script.BaseUrl)
			}
			if script.OpenWPM.Audio {
				log.Log.Infof("Found audio fingerprinting: %s", script.BaseUrl)
			}
			if script.OpenWPM.Battery {
				log.Log.Infof("Found battery fingerprinting: %s", script.BaseUrl)
			}
		}
	}

	return nil
}

func openWPMCheckScript(s *Script) error {

	// Canvas fingerprinting state
	imageExtracted := false
	moreThan10Characters := false
	styles := make(map[string]bool)
	notCanvasFP := false

	//Canvas font fingerprinting state
	fontCalls := 0
	measureTextCalls := 0

	// WebRTC Fingerprinting state
	createDataChannel := false
	createOffer := false
	onIceCandidate := false

	// Audio state
	audioCalls := make(map[string]bool)

	// Battery state
	charging := false
	discharging := false
	level := false

	for _, call := range s.Calls {

		// If we ever set the canvas height or width smaller than 16 pixels, return false
		if call.T == "set" && call.C == "HTMLCanvasElement" && (call.F == "height" || call.F == "width") {
			if len(call.Args) > 0 {
				val, err := strconv.Atoi(call.Args[0].Val)
				if err != nil {
					continue
				}
				if val < 16 {
					notCanvasFP = true
				}
			}
		}

		// The script should not call the save, restore, or addEventListener methods of the rendering context
		if call.C == "CanvasRenderingContext2D" && (call.F == "save" || call.F == "restore") {
			notCanvasFP = true
		}
		if call.C == "HTMLCanvasElement" && call.F == "addEventListener" {
			notCanvasFP = true
		}

		// The script must extract an image with toDataURL or getImageData
		if call.C == "CanvasRenderingContext2D" && call.F == "getImageData" {
			if len(call.Args) >= 4 {
				width, err := strconv.Atoi(call.Args[2].Val)
				if err != nil {
					log.Log.Warn(err)
					continue
				}
				height, err := strconv.Atoi(call.Args[3].Val)
				if err != nil {
					log.Log.Warn(err)
					continue
				}

				if height >= 16 && width >= 16 {
					imageExtracted = true
				}
			}
		}
		if call.C == "HTMLCanvasElement" && call.F == "toDataURL" {
			imageExtracted = true
		}

		// The script must write at least 10 characters or at least 2 colors
		if call.C == "CanvasRenderingContext2D" && (call.F == "fillText" || call.F == "strokeText") {
			if len(call.Args) > 0 {
				text := call.Args[0].Val
				charMap := make(map[rune]bool)
				for _, character := range text {
					charMap[character] = true
				}
				if len(charMap) >= 10 {
					moreThan10Characters = true
				}
			}
		}
		if call.C == "CanvasRenderingContext2D" && (call.F == "fillStyle" || call.F == "strokeStyle") {
			if len(call.Args) > 0 {
				styles[call.Args[0].Val] = true
			}
		}

		// Canvas font fingerprinting checks
		if call.T == "set" && call.C == "CanvasRenderingContext2D" && call.F == "font" {
			fontCalls += 1
		} else if call.C == "CanvasRenderingContext2D" && call.F == "measureText" {
			measureTextCalls += 1
		}

		//WebRTC Checks
		if call.C == "RTCPeerConnection" {
			if call.F == "createDataChannel" {
				createDataChannel = true
			} else if call.F == "createOffer" {
				createOffer = true
			} else if call.F == "onicecandidate" && call.T == "set" {
				onIceCandidate = true
			}
		}

		// Audio Checks
		if call.C == "BaseAudioContext" {
			audioCalls[call.F] = true
		} else if call.C == "OfflineAudioContext" {
			audioCalls[call.F] = true
		} else if call.C == "ScriptProcessorNode" {
			audioCalls[call.F] = true
		}

		// Battery checks
		if call.C == "BatteryManager" {
			if call.F == "level" || call.F == "onlevelchange" {
				level = true
			} else if call.F == "charging" || call.F == "chargingTime" || call.F == "onchargingchange" || call.F == "onchargingtimechance" {
				charging = true
			} else if call.F == "dischargingTime" || call.F == "ondischargingtimechange" {
				discharging = true
			}
		} else if call.C == "EventTarget" && call.F == "addEventListener" {
			if len(call.Args) < 1 {
				continue
			}

			arg := call.Args[0].Val

			if arg == "levelchange" {
				level = true
			} else if arg == "chargingchange" || arg == "chargingtimechange" {
				charging = true
			} else if arg == "dischargingchange" || arg == "dischargingtimechange" {
				discharging = true
			}
		}
	}

	// Canvas fingerprinting
	if imageExtracted && (moreThan10Characters || len(styles) >= 2) && !notCanvasFP {
		s.OpenWPM.Canvas = true
	} else {
		s.OpenWPM.Canvas = false
	}

	// Canvas font fingerprinting
	if measureTextCalls >= 50 && fontCalls >= 50 {
		s.OpenWPM.CanvasFont = true
	} else {
		s.OpenWPM.CanvasFont = false
	}

	// WebRTC fingerprinting
	if createOffer && createDataChannel && onIceCandidate {
		s.OpenWPM.WebRTC = true
	} else {
		s.OpenWPM.WebRTC = false
	}

	// Audio fingerprinting
	s.OpenWPM.Audio = false
	if _, ok := audioCalls["createOscillator"]; ok {
		if _, ok = audioCalls["createDynamicsCompressor"]; ok {
			if _, ok = audioCalls["startRendering"]; ok {
				if _, ok = audioCalls["oncomplete"]; ok {
					s.OpenWPM.Audio = true
				}
			}
		} else if _, ok = audioCalls["createAnalyser"]; ok {
			_, scriptProcessor := audioCalls["scriptProcessor"]
			_, createGain := audioCalls["createGain"]
			_, destination := audioCalls["destination"]
			_, onaudioprocess := audioCalls["onaudioprocess"]

			if scriptProcessor && createGain && destination && onaudioprocess {
				s.OpenWPM.Audio = true
			}
		}
	}

	// Battery fingerprinting
	if charging && discharging && level {
		s.OpenWPM.Battery = true
	} else {
		s.OpenWPM.Battery = false
	}

	return nil
}
