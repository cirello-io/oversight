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
//
// This package is covered by this SLA: https://cirello.io/sla
package oversight // import "cirello.io/oversight"
