package com.poro.app.ui.shop

import androidx.lifecycle.SavedStateHandle
import com.poro.app.R
import com.poro.app.core.network.ApiErrorCodes
import com.poro.app.core.network.ApiResult
import com.poro.app.data.order.Prefill
import com.poro.app.data.order.PrefillDelivery
import com.poro.app.data.order.PreviewDto
import com.poro.app.testing.FakeOrderRepository
import com.poro.app.testing.MainDispatcherRule
import java.io.IOException
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class CheckoutViewModelTest {

    @get:Rule
    val main = MainDispatcherRule()

    private val orders = FakeOrderRepository().apply {
        prefill = ApiResult.Success(Prefill("account", delivery = PrefillDelivery(fullName = "Awa", phone = "+2250701020304", city = "Abidjan")))
    }

    @Test
    fun `the form is prefilled and needs an address before review`() = runTest {
        val vm = CheckoutViewModel(orders, SavedStateHandle())
        runCurrent()

        val form = vm.state.value.form
        assertEquals("Awa", form.fullName)
        assertFalse(form.isValid)
        vm.onFormChange(form.copy(address = "Cocody, rue des jardins"))
        assertTrue(vm.state.value.form.isValid)
    }

    @Test
    fun `a national phone number is refused`() {
        assertFalse(DeliveryForm("Awa", "0701020304", "Abidjan", "Cocody").isValid)
        assertTrue(DeliveryForm("Awa", "+2250701020304", "Abidjan", "Cocody").isValid)
    }

    @Test
    fun `review then confirm places the orders`() = runTest {
        val vm = CheckoutViewModel(orders, SavedStateHandle())
        runCurrent()
        vm.onFormChange(vm.state.value.form.copy(address = "Cocody"))
        vm.onSaveAddressChange(true)

        vm.review()
        runCurrent()
        assertNotNull(vm.state.value.preview)
        assertTrue(orders.lastPreview!!.saveAddress)
        assertEquals("Cocody", orders.lastPreview!!.delivery.address)

        vm.confirm()
        val event = vm.events.first()
        assertEquals("o1", (event as CheckoutEvent.Placed).orders.single().orderId)
        assertTrue("confirm:pv1" in orders.calls)
    }

    @Test
    fun `an outdated summary sends the buyer back to review it`() = runTest {
        orders.preview = ApiResult.Success(PreviewDto("pv1"))
        orders.confirm = ApiResult.Failure(409, ApiErrorCodes.PREVIEW_OUTDATED, "cart changed", null, null)
        val vm = CheckoutViewModel(orders, SavedStateHandle())
        runCurrent()
        vm.onFormChange(vm.state.value.form.copy(address = "Cocody"))
        vm.review()
        runCurrent()

        vm.confirm()
        runCurrent()

        assertNull(vm.state.value.preview)
        assertEquals(R.string.checkout_preview_outdated, vm.state.value.error?.text)
    }

    @Test
    fun `a failed prefill is shown and can be retried`() = runTest {
        orders.prefill = ApiResult.NetworkError(IOException())
        val vm = CheckoutViewModel(orders, SavedStateHandle())
        runCurrent()

        assertFalse(vm.state.value.loading)
        assertEquals(R.string.error_network, vm.state.value.error?.text)

        orders.prefill = ApiResult.Success(Prefill("account", delivery = PrefillDelivery(fullName = "Awa", phone = "+2250701020304", city = "Abidjan")))
        vm.retryLoad()
        runCurrent()

        assertEquals("Awa", vm.state.value.form.fullName)
        assertNull(vm.state.value.error)
    }

    @Test
    fun `a network error on confirm is retried once because the call is idempotent`() = runTest {
        orders.confirmResults += ApiResult.NetworkError(IOException())
        val vm = CheckoutViewModel(orders, SavedStateHandle())
        runCurrent()
        vm.onFormChange(vm.state.value.form.copy(address = "Cocody"))
        vm.review()
        runCurrent()

        vm.confirm()
        val event = vm.events.first()
        assertEquals("o1", (event as CheckoutEvent.Placed).orders.single().orderId)
        assertEquals(2, orders.calls.count { it.startsWith("confirm:") })
    }

    @Test
    fun `two network errors on confirm leave the buyer on the summary`() = runTest {
        orders.confirmResults += ApiResult.NetworkError(IOException())
        orders.confirm = ApiResult.NetworkError(IOException())
        val vm = CheckoutViewModel(orders, SavedStateHandle())
        runCurrent()
        vm.onFormChange(vm.state.value.form.copy(address = "Cocody"))
        vm.review()
        runCurrent()

        vm.confirm()
        runCurrent()

        assertNotNull(vm.state.value.preview)
        assertEquals(R.string.error_network, vm.state.value.error?.text)
        assertFalse(vm.state.value.working)
    }

    @Test
    fun `the form survives process death`() = runTest {
        val saved = SavedStateHandle()
        val first = CheckoutViewModel(orders, saved)
        runCurrent()
        first.onFormChange(DeliveryForm("Kofi", "+2250700000000", "Bouaké", "Quartier Commerce"))

        val restored = CheckoutViewModel(orders, saved)
        runCurrent()

        assertEquals("Kofi", restored.state.value.form.fullName)
        assertEquals("Quartier Commerce", restored.state.value.form.address)
    }
}
