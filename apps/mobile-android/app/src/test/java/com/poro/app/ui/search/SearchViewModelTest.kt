package com.poro.app.ui.search

import androidx.lifecycle.SavedStateHandle
import com.poro.app.core.network.ApiResult
import com.poro.app.data.search.HashtagHit
import com.poro.app.data.search.ProductHit
import com.poro.app.data.search.SearchPage
import com.poro.app.data.search.SearchRepository
import com.poro.app.data.search.ShopHit
import com.poro.app.data.search.Suggestions
import com.poro.app.data.search.UserHit
import com.poro.app.data.search.VideoHit
import com.poro.app.testing.MainDispatcherRule
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.advanceTimeBy
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class SearchViewModelTest {

    @get:Rule
    val main = MainDispatcherRule()

    private val calls = mutableListOf<String>()
    private val repo = object : SearchRepository {
        override suspend fun videos(q: String, cursor: String?): ApiResult<SearchPage<VideoHit>> {
            calls += "videos:$q:$cursor"
            return ApiResult.Success(SearchPage(listOf(VideoHit("v-$cursor")), if (cursor == null) "c2" else null))
        }

        override suspend fun users(q: String, cursor: String?): ApiResult<SearchPage<UserHit>> {
            calls += "users:$q"
            return ApiResult.Success(SearchPage(listOf(UserHit("u1", "kofi"))))
        }

        override suspend fun hashtags(q: String, cursor: String?): ApiResult<SearchPage<HashtagHit>> {
            calls += "hashtags:$q"
            return ApiResult.Success(SearchPage(listOf(HashtagHit("foot", 12))))
        }

        override suspend fun products(q: String, cursor: String?): ApiResult<SearchPage<ProductHit>> {
            calls += "products:$q"
            return ApiResult.Success(SearchPage(listOf(ProductHit("p1", "Pagne wax", minPrice = 9000, currency = "XOF"))))
        }

        override suspend fun shops(q: String, cursor: String?): ApiResult<SearchPage<ShopHit>> {
            calls += "shops:$q"
            return ApiResult.Success(SearchPage(listOf(ShopHit("s1", "Pagnes Awa", "pagnes.awa"))))
        }

        override suspend fun suggest(q: String): ApiResult<Suggestions> {
            calls += "suggest:$q"
            return ApiResult.Success(Suggestions(hashtags = listOf(HashtagHit("foot", 12))))
        }
    }

    @Test
    fun `suggestions wait for a pause in typing`() = runTest(main.dispatcher) {
        val vm = SearchViewModel(repo, SavedStateHandle())

        vm.onQueryChange("fo")
        advanceTimeBy(100)
        vm.onQueryChange("foo")
        advanceTimeBy(400)
        runCurrent()

        assertEquals(listOf("suggest:foo"), calls)
        assertEquals("foot", vm.state.value.suggestions.hashtags.single().tag)
    }

    @Test
    fun `results load per tab, page by page`() = runTest(main.dispatcher) {
        val vm = SearchViewModel(repo, SavedStateHandle())
        vm.onQueryChange("danse")
        vm.submit()
        runCurrent()

        assertTrue("les suggestions s'effacent", vm.state.value.suggestions.hashtags.isEmpty())
        vm.loadMore()
        runCurrent()
        assertEquals(listOf("v-null", "v-c2"), vm.state.value.videos.items.map { it.videoId })

        vm.selectTab(SearchTab.Users)
        vm.selectTab(SearchTab.Videos)
        vm.selectTab(SearchTab.Users)
        runCurrent()
        assertEquals("chaque onglet est chargé une fois", listOf("videos:danse:null", "videos:danse:c2", "users:danse"), calls)
    }

    @Test
    fun `an @username query opens the accounts tab`() = runTest(main.dispatcher) {
        val vm = SearchViewModel(repo, SavedStateHandle())
        vm.submit("@kofi")
        runCurrent()

        assertEquals(SearchTab.Users, vm.state.value.tab)
        assertEquals(listOf("users:@kofi"), calls)
    }

    @Test
    fun `opened from the shop tab, a search starts on products and a handle opens shops`() = runTest(main.dispatcher) {
        val vm = SearchViewModel(repo, SavedStateHandle().apply { set("tab", SearchTab.Products.name) })
        assertEquals(SearchTab.Products, vm.state.value.tab)

        vm.submit("pagne")
        runCurrent()
        assertEquals("p1", vm.state.value.products.items.single().productId)

        vm.submit("@pagnes")
        runCurrent()
        assertEquals("a handle in the catalogue is a shop's", SearchTab.Shops, vm.state.value.tab)
        assertEquals("pagnes.awa", vm.state.value.shops.items.single().handle)
        assertEquals(listOf("products:pagne", "shops:@pagnes"), calls)
    }
}

