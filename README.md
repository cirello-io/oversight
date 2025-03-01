# cirello.io/oversight/v2

[![Go Reference](https://pkg.go.dev/badge/cirello.io/oversight/v2.svg)](https://pkg.go.dev/cirello.io/oversight/v2)

Package oversight makes a complete implementation of the Erlang supervision
trees.

Refer to: http://erlang.org/doc/design_principles/sup_princ.html

go get cirello.io/oversight/v2

https://godoc.org/cirello.io/oversight/v2


## Quickstart
```
var tree oversight.Tree
err := tree.Add(
	oversight.ChildProcessSpecification{
		// Name: "child process",
		Start: func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(time.Second):
				fmt.Println(1)
			}
			return nil
		},
		Restart:  oversight.Permanent(),
		Shutdown: oversight.Infinity(),
	},
)
if err != nil {
	log.Fatal(err)
}
ctx, cancel := context.WithCancel(context.Background())
defer cancel()
fmt.Println(tree.Start(ctx))
```
