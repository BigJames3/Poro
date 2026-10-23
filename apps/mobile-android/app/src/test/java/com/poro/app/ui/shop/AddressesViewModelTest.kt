package com.poro.app.ui.shop

import com.poro.app.core.network.ApiResult
import com.poro.app.data.order.AddressDto
import com.poro.app.testing.FakeOrderRepository
import com.poro.app.testing.MainDispatcherRule
import com.poro.app.ui.common.Loadable
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class AddressesViewModelTest {

    @get:Rule
    val main = MainDispatcherRule()

    private val orders = FakeOrderRepository()

    @Test
    fun `a valid address is created then listed`() = runTest(main.dispatcher) {
        orders.addresses = ApiResult.Success(emptyList())
        val vm = AddressesViewModel(orders)
        runCurrent()

        vm.startCreate()
        vm.onFormChange(DeliveryForm("Awa Kone", "+2250701020304", "Abidjan", "Cocody"))
        vm.onLabelChange("Maison")
        vm.save()
        runCurrent()

        assertEquals(AddressesEvent.Saved, vm.events.first())
        assertTrue("saveAddress:new:Maison" in orders.calls)
        assertTrue(vm.state.value.list is Loadable.Loaded)
    }

    @Test
    fun `an incomplete form is not sent`() = runTest(main.dispatcher) {
        val vm = AddressesViewModel(orders)
        runCurrent()
        vm.startCreate()
        vm.onFormChange(DeliveryForm("Awa", "", "Abidjan"))
        vm.save()
        runCurrent()
        assertTrue(orders.calls.none { it.startsWith("saveAddress") })
    }

    @Test
    fun `delete asks the repository then reloads`() = runTest(main.dispatcher) {
        orders.addresses = ApiResult.Success(listOf(AddressDto("a1", fullName = "Awa", phone = "+2250701020304", city = "Abidjan")))
        val vm = AddressesViewModel(orders)
        runCurrent()
        vm.delete("a1")
        runCurrent()
        assertEquals(AddressesEvent.Deleted, vm.events.first())
        assertTrue("deleteAddress:a1" in orders.calls)
    }
}
