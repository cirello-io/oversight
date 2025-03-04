// Copyright 2018 cirello.io/oversight/v2 - Ulderico Cirello
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

// Package oversight mimics the implementation of Erlang supervision trees.
//
// Refer to: http://erlang.org/doc/design_principles/sup_princ.html
//
//	var tree oversight.Tree
//	err := tree.Add(
//		func(ctx context.Context) error {
//			select {
//			case <-ctx.Done():
//				return nil
//			case <-time.After(time.Second):
//				fmt.Println(1)
//			}
//			return nil
//		},
//		oversight.Permanent(),
//		oversight.Natural(),
//		"childProcess",
//	)
//	if err != nil {
//		log.Fatal(err)
//	}
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//	fmt.Println(tree.Start(ctx))
//
// Security Notice: there has been permanent attempts to use the name of this package to spread malware. Please refer to https://github.com/cirello-io/oversight/issues/3 for more information.
package oversight // import "cirello.io/oversight/v2"
