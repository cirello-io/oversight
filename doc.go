// Copyright 2018 cirello.io/oversight - Ulderico Cirello
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package oversight makes a nearly complete implementation of the Erlang
// supervision trees.
//
// Refer to: http://erlang.org/doc/design_principles/sup_princ.html
//
//     supervisor := oversight.New(
//     	oversight.WithRestartStrategy(oversight.OneForOne()),
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
//     if err := supervisor.Start(ctx); err != nil {
//     	log.Fatal(err)
//     }
//
// Simple interface
//
// If you do not need to use nested trees, you might prefer using
// cirello.io/oversight/easy instead. It provides a OneForAll tree with the
// automatic halting disabled.
//
// 	package main
//
// 	import oversight "cirello.io/oversight/easy"
//
// 	func main() {
// 		ctx, cancel := context.WithCancel(context.Background())
// 		defer cancel() // use cancel() to halt the tree.
// 		ctx = oversight.WithContext(ctx)
// 		oversight.Add(ctx, func(ctx context.Context) {
// 			// ...
// 		})
// 	}
//
// This package is covered by this SLA: https://cirello.io/sla
package oversight // import "cirello.io/oversight"
