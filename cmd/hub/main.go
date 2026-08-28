// Command hub receives measurements, stores them and serves the web page.
package main

import (
	"fmt"

	"github.com/pravbeseda/Monitor/internal/version"
)

func main() {
	fmt.Printf("monitor-hub %s\n", version.Current)
}
