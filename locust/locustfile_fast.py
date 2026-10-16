import random

from locust import FastHttpUser, task

# Same as locustfile.py, but FastHttpUser (geventhttpclient) replaces
# HttpUser (python-requests).

# IDs seeded by product-service.go at startup (seedCount = 1000).
SEEDED_IDS = 1000


class ProductUser(FastHttpUser):
    # No wait_time: each user sends its next request as soon as the last one
    # finishes, so the user count sets how hard the server is pushed.

    # A store is read-heavy: shoppers browse far more than products change,
    # so GET and POST run in a 9:1 ratio.
    @task(9)
    def get_product(self):
        product_id = random.randint(1, SEEDED_IDS)
        self.client.get(f"/products/{product_id}", name="/products/[id]")

    @task(1)
    def add_product_details(self):
        product_id = random.randint(1, SEEDED_IDS)
        self.client.post(
            f"/products/{product_id}/details",
            name="/products/[id]/details",
            json={
                "product_id": product_id,
                "sku": f"SKU-{product_id:05d}",
                "manufacturer": "Locust",
                "category_id": 1,
                "weight": 100,
                "some_other_id": 1,
            },
        )
