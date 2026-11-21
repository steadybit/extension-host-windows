// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

// probe runs in the Windows guest: the spike measures the delay attack through it and reads the extension log from it.
package main

import (
	"net/http"
	"os"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	http.HandleFunc("/log", func(w http.ResponseWriter, _ *http.Request) {
		data, err := os.ReadFile(`C:\steadybit\extension.log`)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		_, _ = w.Write(data)
	})
	_ = http.ListenAndServe(":8080", nil)
}
