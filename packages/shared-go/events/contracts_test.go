package events_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/require"

	"github.com/poro/shared-go/events"
)

const contractsDir = "../../contracts/events"

// contractSamples holds one realistic payload per event type of the catalog.
// Every type must have a sample and a JSON Schema file.
func contractSamples() map[string]any {
	id := uuid.Must(uuid.NewV7()).String()
	user := uuid.Must(uuid.NewV7()).String()
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	ci := "CI"
	username, display, avatar := "awa.kone", "Awa Koné", "https://cdn.poro.test/avatars/a.webp"
	owner, other, parent := uuid.Must(uuid.NewV7()).String(), uuid.Must(uuid.NewV7()).String(), uuid.Must(uuid.NewV7()).String()
	samples := map[string]any{
		events.TypeAuthUserCreated: events.AuthUserCreatedV1{
			UserID: user, SignupMethod: "phone", CountryCode: &ci, Language: "fr", CreatedAt: now,
		},
		events.TypeUserCreatorActivated: events.UserCreatorActivatedV1{
			UserID: user, Username: "awa_k", ActivatedAt: now,
		},
		events.TypeUserProfileUpdated: events.UserProfileUpdatedV1{
			UserID: user, Username: &username, DisplayName: &display, AvatarURL: &avatar, IsCreator: true, UpdatedAt: now,
		},
		events.TypeVideoUploaded: events.VideoUploadedV1{
			VideoID: id, UserID: user, SourceKey: "videos/src.mp4", ContentType: "video/mp4", SizeBytes: 1024, UploadedAt: now,
		},
		events.TypeVideoReady: events.VideoReadyV1{
			VideoID: id, UserID: user, DurationMs: 1500, Width: 720, Height: 1280,
			HLSKey: "hls/master.m3u8", ThumbnailKey: "thumb.jpg",
			Renditions: []events.VideoRenditionV1{{Name: "360p", Bandwidth: 896000, Width: 360, Height: 640, PlaylistKey: "hls/360p/index.m3u8"}},
			ReadyAt:    now, Title: "Danse", Description: "Danse #Abidjan", Hashtags: []string{"abidjan", "coupé_décalé"}, PublishedAt: now,
		},
		events.TypeVideoFailed: events.VideoFailedV1{
			VideoID: id, UserID: user, Code: events.VideoFailTooLong, FailedAt: now,
		},
		events.TypeVideoDeleted: events.VideoDeletedV1{
			VideoID: id, UserID: user, DeletedAt: now,
		},
		events.TypeSocialLikeCreated: events.SocialLikeCreatedV1{
			LikeID: other, UserID: user, VideoID: id, VideoOwnerID: owner, CreatedAt: now,
		},
		events.TypeSocialLikeDeleted: events.SocialLikeDeletedV1{
			UserID: user, VideoID: id, VideoOwnerID: owner, DeletedAt: now,
		},
		events.TypeSocialCommentCreated: events.SocialCommentCreatedV1{
			CommentID: other, UserID: user, VideoID: id, VideoOwnerID: owner,
			ParentID: &parent, ParentAuthorID: &owner, Excerpt: "Trop beau 🔥", Text: "Trop beau 🔥", CreatedAt: now,
		},
		events.TypeSocialCommentUpdated: events.SocialCommentUpdatedV1{
			CommentID: other, UserID: user, VideoID: id, Text: "Trop beau, bravo 🔥", UpdatedAt: now,
		},
		events.TypeSocialCommentDeleted: events.SocialCommentDeletedV1{
			CommentID: other, VideoID: id, UserID: user, DeletedAt: now,
		},
		events.TypeSocialFollowCreated: events.SocialFollowCreatedV1{
			FollowerID: user, FollowingID: owner, CreatedAt: now,
		},
		events.TypeSocialFollowDeleted: events.SocialFollowDeletedV1{
			FollowerID: user, FollowingID: owner, DeletedAt: now,
		},
		events.TypeSocialShareCreated: events.SocialShareCreatedV1{
			ShareID: other, UserID: user, VideoID: id, VideoOwnerID: owner, Channel: events.ShareChannelWhatsApp, CreatedAt: now,
		},
		events.TypeModerationContentRemoved: events.ModerationContentRemovedV1{
			CaseID: other, TargetType: events.ModerationTargetComment, TargetID: id, OwnerID: owner,
			Reason: events.ModerationReasonHarassment, DecidedBy: events.ModerationDecidedByAuto, RemovedAt: now,
		},
		events.TypeModerationContentRestored: events.ModerationContentRestoredV1{
			CaseID: other, TargetType: events.ModerationTargetVideo, TargetID: id, OwnerID: owner, RestoredAt: now,
		},
	}
	for typ, data := range marketplaceSamples(now) {
		samples[typ] = data
	}
	return samples
}

func marketplaceSamples(now time.Time) map[string]any {
	shop, product, variant := uuid.Must(uuid.NewV7()).String(), uuid.Must(uuid.NewV7()).String(), uuid.Must(uuid.NewV7()).String()
	order, seller, buyer := uuid.Must(uuid.NewV7()).String(), uuid.Must(uuid.NewV7()).String(), uuid.Must(uuid.NewV7()).String()
	payment, refund := uuid.Must(uuid.NewV7()).String(), uuid.Must(uuid.NewV7()).String()
	desc, logo, tracking := "Pagnes tissés à la main", "https://cdn.poro.test/shops/logo.webp", "Colis remis à Yango"
	expires := now.Add(30 * time.Minute)
	return map[string]any{
		events.TypeShopShopCreated: events.ShopShopCreatedV1{
			ShopID: shop, OwnerID: seller, Name: "Pagnes d'Awa", Handle: "pagnes.awa", CountryCode: "CI",
			Currency: events.CurrencyXOF, CreatedAt: now,
		},
		events.TypeShopShopUpdated: events.ShopShopUpdatedV1{
			ShopID: shop, OwnerID: seller, Name: "Pagnes d'Awa", Handle: "pagnes.awa", Description: &desc, LogoURL: &logo,
			CountryCode: "CI", Currency: events.CurrencyXOF, Status: events.ShopStatusActive, UpdatedAt: now,
		},
		events.TypeShopProductUpdated: events.ShopProductUpdatedV1{
			ProductID: product, ShopID: shop, OwnerID: seller, Title: "Pagne wax 6 yards", Description: &desc,
			Currency: events.CurrencyXOF, Status: events.ProductStatusActive,
			ImageURLs: []string{"https://cdn.poro.test/products/p1.webp"},
			Variants:  []events.ShopVariantV1{{VariantID: variant, Title: "Bleu", Price: 15000, InStock: true}},
			UpdatedAt: now,
		},
		events.TypeShopProductDeleted: events.ShopProductDeletedV1{
			ProductID: product, ShopID: shop, OwnerID: seller, DeletedAt: now,
		},
		events.TypeOrderOrderCreated: events.OrderOrderCreatedV1{
			OrderID: order, BuyerID: buyer, ShopID: shop, SellerID: seller, Currency: events.CurrencyXOF,
			PaymentMethod: events.PaymentMethodWave,
			Items:         []events.OrderLineV1{{ProductID: product, VariantID: variant, Quantity: 2, UnitPrice: 15000}},
			CreatedAt:     now,
		},
		events.TypeShopStockReserved: events.ShopStockReservedV1{
			OrderID: order, ShopID: shop, Currency: events.CurrencyXOF,
			Items:    []events.ShopReservedLineV1{{ProductID: product, VariantID: variant, Quantity: 2, UnitPrice: 15000, Title: "Pagne wax 6 yards — Bleu"}},
			Subtotal: 30000, ReservedAt: now,
		},
		events.TypeShopStockRejected: events.ShopStockRejectedV1{
			OrderID: order, ShopID: shop, Reason: events.StockRejectedOutOfStock, VariantIDs: []string{variant}, RejectedAt: now,
		},
		events.TypeOrderOrderPlaced: events.OrderOrderPlacedV1{
			OrderID: order, BuyerID: buyer, ShopID: shop, SellerID: seller, Currency: events.CurrencyXOF, Total: 30000,
			PaymentMethod: events.PaymentMethodWave, PlacedAt: now, ExpiresAt: &expires,
		},
		events.TypeOrderOrderCancelled: events.OrderOrderCancelledV1{
			OrderID: order, BuyerID: buyer, ShopID: shop, SellerID: seller, Reason: events.OrderCancelledSellerCancelled,
			Paid: true, CancelledAt: now,
		},
		events.TypeOrderOrderShipped: events.OrderOrderShippedV1{
			OrderID: order, BuyerID: buyer, ShopID: shop, SellerID: seller, Tracking: &tracking, ShippedAt: now,
		},
		events.TypeOrderOrderCompleted: events.OrderOrderCompletedV1{
			OrderID: order, BuyerID: buyer, ShopID: shop, SellerID: seller, Currency: events.CurrencyXOF, Total: 30000,
			PaymentMethod: events.PaymentMethodWave, CompletedAt: now,
		},
		events.TypePaymentSucceeded: events.PaymentSucceededV1{
			PaymentID: payment, OrderID: order, BuyerID: buyer, Amount: 30000, Currency: events.CurrencyXOF,
			Provider: events.PaymentMethodWave, SucceededAt: now,
		},
		events.TypePaymentFailed: events.PaymentFailedV1{
			PaymentID: payment, OrderID: order, BuyerID: buyer, Provider: events.PaymentMethodWave,
			Reason: events.PaymentFailedDeclined, FailedAt: now,
		},
		events.TypePaymentRefundSucceeded: events.PaymentRefundSucceededV1{
			RefundID: refund, PaymentID: payment, OrderID: order, BuyerID: buyer, Amount: 30000,
			Currency: events.CurrencyXOF, Provider: events.PaymentMethodWave, RefundedAt: now,
		},
	}
}

func compileContract(t *testing.T, eventType string) *jsonschema.Schema {
	t.Helper()
	path := filepath.Join(contractsDir, eventType+".v1.json")
	raw, err := os.ReadFile(path)
	require.NoError(t, err, "every event type needs a contract file")
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	require.NoError(t, err)
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	require.NoError(t, c.AddResource(path, doc))
	schema, err := c.Compile(path)
	require.NoError(t, err)
	return schema
}

func validate(t *testing.T, schema *jsonschema.Schema, v any) error {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	require.NoError(t, err)
	return schema.Validate(inst)
}

func TestCatalogMatchesContracts(t *testing.T) {
	for eventType, data := range contractSamples() {
		t.Run(eventType, func(t *testing.T) {
			schema := compileContract(t, eventType)
			env, err := events.New(eventType, 1, "test", uuid.Must(uuid.NewV7()).String(), data, time.Now())
			require.NoError(t, err)
			require.NoError(t, validate(t, schema, env))
		})
	}
}

func TestEveryContractHasASample(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(contractsDir, "*.v1.json"))
	require.NoError(t, err)
	require.NotEmpty(t, files)
	samples := contractSamples()
	for _, f := range files {
		eventType := filepath.Base(f[:len(f)-len(".v1.json")])
		_, ok := samples[eventType]
		require.True(t, ok, "contract %s has no Go sample", eventType)
	}
}

// Older video.ready events predate title, description, hashtags and published_at.
func TestVideoReadyWithoutLateFieldsStaysValid(t *testing.T) {
	schema := compileContract(t, events.TypeVideoReady)
	now := time.Now().UTC()
	legacy := map[string]any{
		"video_id": uuid.Must(uuid.NewV7()).String(), "user_id": uuid.Must(uuid.NewV7()).String(),
		"duration_ms": 1, "width": 1, "height": 1, "hls_key": "h", "thumbnail_key": "t",
		"renditions": []map[string]any{{"name": "360p", "bandwidth": 1, "width": 1, "height": 1, "playlist_key": "p"}},
		"ready_at":   now,
	}
	env, err := events.New(events.TypeVideoReady, 1, "media-worker", uuid.Must(uuid.NewV7()).String(), legacy, now)
	require.NoError(t, err)
	require.NoError(t, validate(t, schema, env))
}

func TestVideoReadyRejectsBadHashtag(t *testing.T) {
	schema := compileContract(t, events.TypeVideoReady)
	data := contractSamples()[events.TypeVideoReady].(events.VideoReadyV1)
	data.Hashtags = []string{"#abidjan"}
	env, err := events.New(events.TypeVideoReady, 1, "media-worker", data.VideoID, data, time.Now())
	require.NoError(t, err)
	require.Error(t, validate(t, schema, env))
}

// A profile without a username yet still publishes a valid snapshot.
func TestUserProfileUpdatedAllowsEmptyProfile(t *testing.T) {
	schema := compileContract(t, events.TypeUserProfileUpdated)
	user := uuid.Must(uuid.NewV7()).String()
	env, err := events.New(events.TypeUserProfileUpdated, 1, "poro-user", user,
		events.UserProfileUpdatedV1{UserID: user, UpdatedAt: time.Now()}, time.Now())
	require.NoError(t, err)
	require.NoError(t, validate(t, schema, env))
}

func TestSocialContractsRejectBadPayloads(t *testing.T) {
	samples := contractSamples()
	topLevel := samples[events.TypeSocialCommentCreated].(events.SocialCommentCreatedV1)
	topLevel.ParentID, topLevel.ParentAuthorID = nil, nil

	longExcerpt := samples[events.TypeSocialCommentCreated].(events.SocialCommentCreatedV1)
	longExcerpt.Excerpt = strings.Repeat("a", 141)

	badChannel := samples[events.TypeSocialShareCreated].(events.SocialShareCreatedV1)
	badChannel.Channel = "telegram"

	notUUID := samples[events.TypeSocialFollowCreated].(events.SocialFollowCreatedV1)
	notUUID.FollowingID = "awa"

	withoutText := samples[events.TypeSocialCommentCreated].(events.SocialCommentCreatedV1)
	withoutText.Text = ""

	longText := samples[events.TypeSocialCommentUpdated].(events.SocialCommentUpdatedV1)
	longText.Text = strings.Repeat("a", 1001)

	badReason := samples[events.TypeModerationContentRemoved].(events.ModerationContentRemovedV1)
	badReason.Reason = "boring"

	restoredComment := samples[events.TypeModerationContentRestored].(events.ModerationContentRestoredV1)
	restoredComment.TargetType = events.ModerationTargetComment

	cases := []struct {
		name  string
		typ   string
		data  any
		valid bool
	}{
		{"top-level comment has null parent", events.TypeSocialCommentCreated, topLevel, true},
		{"excerpt longer than 140", events.TypeSocialCommentCreated, longExcerpt, false},
		{"unknown share channel", events.TypeSocialShareCreated, badChannel, false},
		{"following_id must be a uuid", events.TypeSocialFollowCreated, notUUID, false},
		{"comment without text predates the field", events.TypeSocialCommentCreated, withoutText, true},
		{"edited text longer than 1000", events.TypeSocialCommentUpdated, longText, false},
		{"unknown removal reason", events.TypeModerationContentRemoved, badReason, false},
		{"comments cannot be restored", events.TypeModerationContentRestored, restoredComment, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			schema := compileContract(t, tc.typ)
			env, err := events.New(tc.typ, 1, "social", uuid.Must(uuid.NewV7()).String(), tc.data, time.Now())
			require.NoError(t, err)
			err = validate(t, schema, env)
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestMarketplaceContractsRejectBadPayloads(t *testing.T) {
	samples := contractSamples()
	euros := samples[events.TypeOrderOrderCreated].(events.OrderOrderCreatedV1)
	euros.Currency = "EUR"

	emptyOrder := samples[events.TypeOrderOrderCreated].(events.OrderOrderCreatedV1)
	emptyOrder.Items = nil

	tooMany := samples[events.TypeOrderOrderCreated].(events.OrderOrderCreatedV1)
	tooMany.Items = []events.OrderLineV1{{ProductID: tooMany.Items[0].ProductID, VariantID: tooMany.Items[0].VariantID, Quantity: 100}}

	negative := samples[events.TypeShopStockReserved].(events.ShopStockReservedV1)
	negative.Subtotal = -1

	ghana := samples[events.TypeShopShopCreated].(events.ShopShopCreatedV1)
	ghana.CountryCode = "GH"

	noVariant := samples[events.TypeShopProductUpdated].(events.ShopProductUpdatedV1)
	noVariant.Variants = nil

	codProvider := samples[events.TypePaymentSucceeded].(events.PaymentSucceededV1)
	codProvider.Provider = events.PaymentMethodCashOnDelivery

	cod := samples[events.TypeOrderOrderPlaced].(events.OrderOrderPlacedV1)
	cod.PaymentMethod, cod.ExpiresAt = events.PaymentMethodCashOnDelivery, nil

	wholeShop := samples[events.TypeShopStockRejected].(events.ShopStockRejectedV1)
	wholeShop.Reason, wholeShop.VariantIDs = events.StockRejectedShopUnavailable, []string{}

	cases := []struct {
		name  string
		typ   string
		data  any
		valid bool
	}{
		{"unsupported currency", events.TypeOrderOrderCreated, euros, false},
		{"order without lines", events.TypeOrderOrderCreated, emptyOrder, false},
		{"quantity above 99", events.TypeOrderOrderCreated, tooMany, false},
		{"negative amount", events.TypeShopStockReserved, negative, false},
		{"country outside the launch list", events.TypeShopShopCreated, ghana, false},
		{"product without variant", events.TypeShopProductUpdated, noVariant, false},
		{"cash on delivery is not a payment provider", events.TypePaymentSucceeded, codProvider, false},
		{"cash on delivery never expires", events.TypeOrderOrderPlaced, cod, true},
		{"a whole-shop rejection names no variant", events.TypeShopStockRejected, wholeShop, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			schema := compileContract(t, tc.typ)
			env, err := events.New(tc.typ, 1, "test", uuid.Must(uuid.NewV7()).String(), tc.data, time.Now())
			require.NoError(t, err)
			err = validate(t, schema, env)
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}
