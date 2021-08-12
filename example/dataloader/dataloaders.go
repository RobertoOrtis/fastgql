//go:generate go run github.com/vektah/dataloaden AddressLoader int *github.com/RobertoOrtis/fastgql/example/dataloader.Address
//go:generate go run github.com/vektah/dataloaden OrderSliceLoader int []*github.com/RobertoOrtis/fastgql/example/dataloader.Order
//go:generate go run github.com/vektah/dataloaden ItemSliceLoader int []*github.com/RobertoOrtis/fastgql/example/dataloader.Item

package dataloader

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/valyala/fasthttp"
)

var ctxKey = "userCtx"

type loaders struct {
	addressByID      *AddressLoader
	ordersByCustomer *OrderSliceLoader
	itemsByOrder     *ItemSliceLoader
}

// nolint: gosec
func LoaderMiddleware(ctx fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {

		ldrs := loaders{}

		// set this to zero what happens without dataloading
		wait := 250 * time.Microsecond

		// simple 1:1 loader, fetch an address by its primary key
		ldrs.addressByID = &AddressLoader{
			wait:     wait,
			maxBatch: 100,
			fetch: func(keys []int) ([]*Address, []error) {
				var keySql []string
				for _, key := range keys {
					keySql = append(keySql, strconv.Itoa(key))
				}

				fmt.Printf("SELECT * FROM address WHERE id IN (%s)\n", strings.Join(keySql, ","))
				time.Sleep(5 * time.Millisecond)

				addresses := make([]*Address, len(keys))
				errors := make([]error, len(keys))
				for i, key := range keys {
					addresses[i] = &Address{Street: "home street", Country: "hometon " + strconv.Itoa(key)}
				}
				return addresses, errors
			},
		}

		// 1:M loader
		ldrs.ordersByCustomer = &OrderSliceLoader{
			wait:     wait,
			maxBatch: 100,
			fetch: func(keys []int) ([][]*Order, []error) {
				var keySql []string
				for _, key := range keys {
					keySql = append(keySql, strconv.Itoa(key))
				}

				fmt.Printf("SELECT * FROM orders WHERE customer_id IN (%s)\n", strings.Join(keySql, ","))
				time.Sleep(5 * time.Millisecond)

				orders := make([][]*Order, len(keys))
				errors := make([]error, len(keys))
				for i, key := range keys {
					id := 10 + rand.Int()%3
					orders[i] = []*Order{
						{ID: id, Amount: rand.Float64(), Date: time.Now().Add(-time.Duration(key) * time.Hour)},
						{ID: id + 1, Amount: rand.Float64(), Date: time.Now().Add(-time.Duration(key) * time.Hour)},
					}

					// if you had another customer loader you would prime its cache here
					// by calling `ldrs.ordersByID.Prime(id, orders[i])`
				}

				return orders, errors
			},
		}

		// M:M loader
		ldrs.itemsByOrder = &ItemSliceLoader{
			wait:     wait,
			maxBatch: 100,
			fetch: func(keys []int) ([][]*Item, []error) {
				var keySql []string
				for _, key := range keys {
					keySql = append(keySql, strconv.Itoa(key))
				}

				fmt.Printf("SELECT * FROM items JOIN item_order WHERE item_order.order_id IN (%s)\n", strings.Join(keySql, ","))
				time.Sleep(5 * time.Millisecond)

				items := make([][]*Item, len(keys))
				errors := make([]error, len(keys))
				for i := range keys {
					items[i] = []*Item{
						{Name: "item " + strconv.Itoa(rand.Int()%20+20)},
						{Name: "item " + strconv.Itoa(rand.Int()%20+20)},
					}
				}

				return items, errors
			},
		}

		ctx.SetUserValue(ctxKey, ldrs)
	}
}

func ctxLoaders(ctx context.Context) loaders {
	return ctx.Value(ctxKey).(loaders)
}
