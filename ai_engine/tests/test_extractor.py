import asyncio
from src.services.intent_service import intent_service
from src.schemas.intents import ParseIntentRequest

def main():
    request = ParseIntentRequest(prompt="create a sale order for eslam, add the product Dp 4 filled Case and deliver it")
    res = intent_service.parse_intent(request)
    print(res.model_dump_json(indent=2))

if __name__ == "__main__":
    main()
