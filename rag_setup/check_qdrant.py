import qdrant_client
client = qdrant_client.QdrantClient("http://localhost:6333")
try:
    print(client.get_collection("books").model_dump_json(indent=2))
except Exception as e:
    print(e)
