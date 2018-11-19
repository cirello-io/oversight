// Package oversight makes a nearly complete implementation of the Erlang
// supervision trees.
//
// Refer to: http://erlang.org/doc/design_principles/sup_princ.html
//
//     supervise := oversight.Oversight(
//     	oversight.Processes(func(ctx context.Context) error {
//     		select {
//     		case <-ctx.Done():
//     			return nil
//     		case <-time.After(time.Second):
//     			log.Println(1)
//     		}
//     		return nil
//     	}),
//     )
//     ctx, cancel := context.WithCancel(context.Background())
//     defer cancel()
//     if err := supervise(ctx); err != nil {
//     	log.Fatal(err)
//     }
package oversight // import "cirello.io/oversight"
