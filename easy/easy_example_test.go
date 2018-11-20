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

package easy_test

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	oversight "cirello.io/oversight/easy"
)

func Example() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	ctx = oversight.WithContext(ctx)
	wg.Add(1)
	serviceName, err := oversight.Add(ctx, func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return nil
		default:
			defer wg.Done()
			fmt.Println("executed successfully")
			cancel()
			return nil
		}
	})
	if err != nil {
		log.Fatal(err)
	}

	wg.Wait()

	if err := oversight.Delete(ctx, serviceName); err != nil {
		log.Fatal(err)
	}

	// Output:
	// executed successfully
}
