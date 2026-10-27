from rapidfuzz import process, fuzz
from typing import List, Dict, Any

class MPesaReconciler:
    @staticmethod
    def match_statement_to_invoices(
        mpesa_narration: str, 
        mpesa_amount: float, 
        open_invoices: List[Dict[str, Any]], 
        threshold: float = 80.0
    ) -> List[Dict[str, Any]]:
        """
        Matches a raw M-Pesa narrative (e.g., 'Conf QX789 Paid by JOHN DOE for INV-1049')
        against open ERP invoices.
        """
        matches = []
        for inv in open_invoices:
            # 1. Exact or near-exact Amount Match
            if abs(inv.get("amount_due", 0.0) - mpesa_amount) < 1.0:
                # 2. Fuzzy Text Score against Customer Name and Invoice Number
                score_inv = fuzz.partial_ratio(str(inv.get("invoice_no", "")).lower(), mpesa_narration.lower())
                score_name = fuzz.token_set_ratio(str(inv.get("customer_name", "")).lower(), mpesa_narration.lower())
                
                combined_score = (score_inv * 0.6) + (score_name * 0.4)
                
                if combined_score >= threshold:
                    matches.append({
                        "invoice_id": inv.get("id"),
                        "invoice_no": inv.get("invoice_no"),
                        "confidence_score": combined_score,
                        "matched_amount": mpesa_amount
                    })
                    
        return sorted(matches, key=lambda x: x["confidence_score"], reverse=True)
