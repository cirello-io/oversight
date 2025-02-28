# cirello.io/oversight/v2/easy

[![Go Reference](https://pkg.go.dev/badge/cirello.io/oversight/v2/easy.svg)](https://pkg.go.dev/cirello.io/oversight/v2/easy)

Package easy is an easier interface to use cirello.io/oversight/v2. Its lifecycle
is managed through context.Context. Stop a given oversight tree by cancelling
its context.

go get cirello.io/oversight/v2/easy

http://godoc.org/cirello.io/oversight/v2/easy


## Quickstart

```
package main

import oversight "cirello.io/oversight/v2/easy"

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
