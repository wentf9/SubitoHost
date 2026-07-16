module github.com/wentf9/subitohost

go 1.26.1

// PulseAudio may request slightly more data than the negotiated buffer maximum.
// Pin the upstream fix from https://github.com/jfreymuth/pulse/pull/46.
replace github.com/jfreymuth/pulse => github.com/wentf9/pulse v0.1.3-0.20260716142017-1a83b1de0675

require (
	github.com/ebitengine/oto/v3 v3.5.0-alpha.8
	github.com/gorilla/websocket v1.5.3
	github.com/jfreymuth/pulse v0.1.2
	github.com/spf13/cobra v1.10.2
)

require (
	github.com/ebitengine/purego v0.10.1 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	golang.org/x/sys v0.45.0 // indirect
)
