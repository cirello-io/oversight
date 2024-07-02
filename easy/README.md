# cirello.io/oversight/easy

[![Go Reference](https://pkg.go.dev/badge/cirello.io/oversight/easy.svg)](https://pkg.go.dev/cirello.io/oversight/easy)

Package easy is an easier interface to use cirello.io/oversight. Its lifecycle
is managed through context.Context. Stop a given oversight tree by cancelling
its context.

go get [-u -f] cirello.io/oversight/easy

http://godoc.org/cirello.io/oversight/easy


## Quickstart

```
package main

import oversight "cirello.io/oversight/easy"

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// use cancel() to stop the oversight
	ctx = oversight.WithContext(ctx)
	oversight.Add(ctx, func(ctx context.Context) {
		// ...
	})
}
```