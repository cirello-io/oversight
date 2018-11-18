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
