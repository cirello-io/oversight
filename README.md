# cirello.io/oversight/v2

[![Go Reference](https://pkg.go.dev/badge/cirello.io/oversight/v2.svg)](https://pkg.go.dev/cirello.io/oversight/v2)

Package oversight makes a complete implementation of the Erlang supervision
trees.

Refer to: http://erlang.org/doc/design_principles/sup_princ.html

go get cirello.io/oversight/v2

https://godoc.org/cirello.io/oversight/v2


## Quickstart
```
supervise := oversight.New(
	oversight.Processes(func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(time.Second):
			log.Println(1)
		}
		return nil
	}),
)

ctx, cancel := context.WithCancel(context.Background())
defer cancel()
if err := supervise.Start(ctx); err != nil {
	log.Fatal(err)
}
```
