package consumer_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/poro/shared-go/events"

	"github.com/poro/search/internal/consumer"
	"github.com/poro/search/internal/index"
	"github.com/poro/search/internal/testdb"
)

type listing struct {
	shop, owner, product uuid.UUID
	word                 string
}

func newListing() listing {
	return listing{shop: uuid.New(), owner: uuid.New(), product: uuid.New(), word: "wax" + uuid.NewString()[:8]}
}

func (l listing) shopEvent(status, country string, at time.Time) events.ShopShopUpdatedV1 {
	return events.ShopShopUpdatedV1{ShopID: l.shop.String(), OwnerID: l.owner.String(), Name: "Pagnes Awa",
		Handle: "pagnes." + l.word, CountryCode: country, Currency: "XOF", Status: status, UpdatedAt: at}
}

func (l listing) productEvent(status string, prices []int64, at time.Time) events.ShopProductUpdatedV1 {
	variants := make([]events.ShopVariantV1, 0, len(prices))
	for i, p := range prices {
		variants = append(variants, events.ShopVariantV1{VariantID: uuid.NewString(), Title: "V", Price: p, InStock: i == 0})
	}
	return events.ShopProductUpdatedV1{ProductID: l.product.String(), ShopID: l.shop.String(), OwnerID: l.owner.String(),
		Title: "Pagne " + l.word, Currency: "XOF", Status: status, ImageURLs: []string{"https://cdn.poro.test/a.webp", "b"},
		Variants: variants, UpdatedAt: at}
}

func products(t *testing.T, q string, f index.CatalogFilter) []index.ProductHit {
	t.Helper()
	page, err := index.NewPostgres(testdb.Pool).Products(context.Background(), q, f, 0, 50)
	require.NoError(t, err)
	return page.Items
}

func shops(t *testing.T, q string, f index.CatalogFilter) []index.ShopHit {
	t.Helper()
	page, err := index.NewPostgres(testdb.Pool).Shops(context.Background(), q, f, 0, 50)
	require.NoError(t, err)
	return page.Items
}

func TestCatalogFindsActiveProductsOfActiveShops(t *testing.T) {
	testdb.Available(t)
	l := newListing()
	t0 := time.Now().UTC()
	send(t, events.TypeShopShopUpdated, l.shopEvent("active", "CI", t0))
	send(t, events.TypeShopProductUpdated, l.productEvent("active", []int64{15000, 9000}, t0))

	got := products(t, l.word, index.CatalogFilter{})
	require.Len(t, got, 1)
	require.Equal(t, l.product, got[0].ProductID)
	require.EqualValues(t, 9000, *got[0].MinPrice, "the lowest variant price")
	require.True(t, got[0].InStock)
	require.Equal(t, "https://cdn.poro.test/a.webp", *got[0].ImageURL, "the first image")
	require.Equal(t, "CI", got[0].Shop.CountryCode)
	require.Len(t, shops(t, "@pagnes."+l.word, index.CatalogFilter{}), 1, "a shop is found by its handle")

	// Filters.
	ci, sn, xof := "CI", "SN", "XOF"
	low, high := int64(10000), int64(8000)
	require.Len(t, products(t, l.word, index.CatalogFilter{CountryCode: &ci, Currency: &xof}), 1)
	require.Empty(t, products(t, l.word, index.CatalogFilter{CountryCode: &sn}))
	require.Empty(t, products(t, l.word, index.CatalogFilter{MinPrice: &low}), "9000 is below 10000")
	require.Empty(t, products(t, l.word, index.CatalogFilter{MaxPrice: &high}), "9000 is above 8000")
	require.Empty(t, shops(t, "pagnes."+l.word, index.CatalogFilter{CountryCode: &sn}))

	// An older snapshot loses; an archived product disappears.
	send(t, events.TypeShopProductUpdated, l.productEvent("archived", []int64{1}, t0.Add(-time.Minute)))
	require.Len(t, products(t, l.word, index.CatalogFilter{}), 1, "an older snapshot is ignored")
	send(t, events.TypeShopProductUpdated, l.productEvent("archived", []int64{9000}, t0.Add(time.Second)))
	require.Empty(t, products(t, l.word, index.CatalogFilter{}))
	send(t, events.TypeShopProductUpdated, l.productEvent("active", []int64{9000}, t0.Add(2*time.Second)))
	require.Len(t, products(t, l.word, index.CatalogFilter{}), 1)

	// A suspended shop hides itself and its products.
	send(t, events.TypeShopShopUpdated, l.shopEvent("suspended", "CI", t0.Add(3*time.Second)))
	require.Empty(t, products(t, l.word, index.CatalogFilter{}))
	require.Empty(t, shops(t, "pagnes."+l.word, index.CatalogFilter{}))
	send(t, events.TypeShopShopUpdated, l.shopEvent("active", "CI", t0.Add(4*time.Second)))
	require.Len(t, products(t, l.word, index.CatalogFilter{}), 1, "a reinstated shop is found again")

	// A deleted product never comes back, even with a later snapshot.
	send(t, events.TypeShopProductDeleted, events.ShopProductDeletedV1{ProductID: l.product.String(),
		ShopID: l.shop.String(), OwnerID: l.owner.String(), DeletedAt: t0.Add(5 * time.Second)})
	require.Empty(t, products(t, l.word, index.CatalogFilter{}))
	send(t, events.TypeShopProductUpdated, l.productEvent("active", []int64{9000}, t0.Add(6*time.Second)))
	require.Empty(t, products(t, l.word, index.CatalogFilter{}))
}

func TestErasureRemovesTheCatalogOfTheAccount(t *testing.T) {
	testdb.Available(t)
	l := newListing()
	now := time.Now().UTC()
	send(t, events.TypeShopShopUpdated, l.shopEvent("active", "CI", now))
	send(t, events.TypeShopProductUpdated, l.productEvent("active", []int64{5000}, now))
	require.Len(t, products(t, l.word, index.CatalogFilter{}), 1)

	env, err := events.New(events.TypeAuthUserDeleted, 1, "test", uuid.NewString(),
		events.AuthUserDeletedV1{UserID: l.owner.String(), DeletedAt: now}, now)
	require.NoError(t, err)
	require.NoError(t, consumer.NewEraser(testdb.Pool, zap.NewNop())(context.Background(), env))
	require.Empty(t, products(t, l.word, index.CatalogFilter{}))
	require.Empty(t, shops(t, "pagnes."+l.word, index.CatalogFilter{}))

	send(t, events.TypeShopShopUpdated, l.shopEvent("active", "CI", now.Add(time.Minute)))
	send(t, events.TypeShopProductUpdated, l.productEvent("active", []int64{5000}, now.Add(time.Minute)))
	require.Empty(t, products(t, l.word, index.CatalogFilter{}), "a late event never indexes an erased account again")
}
