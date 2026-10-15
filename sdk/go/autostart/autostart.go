// Package autostart starts pamone on import (logs + metrics):
//
//	import _ "github.com/bennnr/pamone-observability/sdk/go/autostart"
//
// Configure with PAMONE_* env vars; traces additionally need the app to add
// pamone.NewSpanProcessor() to its OpenTelemetry TracerProvider.
package autostart

import (
	pamone "github.com/bennnr/pamone-observability/sdk/go/pamone"
)

func init() { pamone.Init() }
