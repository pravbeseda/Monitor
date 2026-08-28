// Command agent collects measurements on a node and pushes them to the hub.
package main

import (
	"fmt"

	"github.com/pravbeseda/Monitor/internal/version"
)

func main() {
	fmt.Printf("monitor-agent %s\n", version.Current)
}
