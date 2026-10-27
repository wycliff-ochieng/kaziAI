import os

import pytest
import unittest
from src.schemas.intents import ParseIntentRequest, IntentType, ActionType
from src.services.intent_service import intent_service

pytestmark = pytest.mark.skipif(
    not os.getenv("RUN_LLM_TESTS"),
    reason="requires a live LLM provider",
)


class TestIntentParsing(unittest.TestCase):

    def test_parse_get_customers_intent(self):
        req = ParseIntentRequest(prompt="Show me all active customers")
        res = intent_service.parse_intent(req)
        self.assertEqual(res.intent_type, IntentType.GET_CUSTOMERS)
        self.assertEqual(res.action_type, ActionType.READ)

    def test_parse_create_quotation_intent(self):
        req = ParseIntentRequest(prompt="Draft a quotation for Azure Interior for 10 units of Ergonomic Office Chair")
        res = intent_service.parse_intent(req)
        self.assertEqual(res.intent_type, IntentType.CREATE_QUOTATION)
        self.assertEqual(res.action_type, ActionType.WRITE)
        self.assertEqual(res.parameters.customer_name, "Azure Interior")
        self.assertEqual(res.parameters.quantity, 10.0)
        self.assertTrue(res.validation.requires_user_approval)
        self.assertIn("Would you like to confirm and generate this Quotation", res.validation.approval_prompt)

    def test_parse_create_sale_order_intent(self):
        req = ParseIntentRequest(prompt="Create a sale order for Azure Interior for 5 units of Customizable Desk")
        res = intent_service.parse_intent(req)
        self.assertEqual(res.intent_type, IntentType.CREATE_SALE_ORDER)
        self.assertEqual(res.action_type, ActionType.WRITE)
        self.assertEqual(res.parameters.customer_name, "Azure Interior")
        self.assertEqual(res.parameters.quantity, 5.0)
        self.assertTrue(res.validation.requires_user_approval)

    def test_parse_confirm_sale_order_intent(self):
        req = ParseIntentRequest(prompt="Confirm sale order SO001")
        res = intent_service.parse_intent(req)
        self.assertEqual(res.intent_type, IntentType.CONFIRM_SALE_ORDER)
        self.assertEqual(res.action_type, ActionType.WRITE)
        self.assertEqual(res.parameters.order_id, "SO001")

    def test_parse_deliver_sale_order_intent(self):
        req = ParseIntentRequest(prompt="Process delivery for sale order SO001")
        res = intent_service.parse_intent(req)
        self.assertEqual(res.intent_type, IntentType.DELIVER_SALE_ORDER)
        self.assertEqual(res.action_type, ActionType.WRITE)
        self.assertEqual(res.parameters.order_id, "SO001")
        self.assertEqual(res.validation.risk_level, "HIGH")

    def test_parse_credit_note_intent(self):
        req = ParseIntentRequest(prompt="Issue a credit note for Invoice INV/2026/0001 for 200 due to damaged goods")
        res = intent_service.parse_intent(req)
        self.assertEqual(res.intent_type, IntentType.CREATE_CREDIT_NOTE)
        self.assertEqual(res.action_type, ActionType.WRITE)
        self.assertEqual(res.parameters.invoice_id, "INV/2026/0001")
        self.assertEqual(res.parameters.amount, 200.0)

    def test_parse_etims_fiscalize_intent(self):
        req = ParseIntentRequest(prompt="Submit invoice INV/2026/0001 to eTIMS for fiscalization")
        res = intent_service.parse_intent(req)
        self.assertEqual(res.intent_type, IntentType.SUBMIT_ETIMS)
        self.assertEqual(res.action_type, ActionType.WRITE)
        self.assertEqual(res.parameters.invoice_id, "INV/2026/0001")

    def test_parse_print_document_intent(self):
        req = ParseIntentRequest(prompt="Print pdf quotation for SO001")
        res = intent_service.parse_intent(req)
        self.assertEqual(res.intent_type, IntentType.PRINT_DOCUMENT)
        self.assertEqual(res.action_type, ActionType.READ)
        self.assertEqual(res.parameters.document_type, "quotation")

if __name__ == "__main__":
    unittest.main()
