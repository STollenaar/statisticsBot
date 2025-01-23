from pymilvus import connections, CollectionSchema, FieldSchema, DataType, Collection

# Connect to Milvus
connections.connect("default", host="localhost", port="19530")

# Define the new schema
new_schema = CollectionSchema(
    fields=[
        FieldSchema(name="id", dtype=DataType.VARCHAR, is_primary=True, max_length=64),
        FieldSchema(name="guild_id", dtype=DataType.VARCHAR, is_primary=False, max_length=64),
        FieldSchema(name="channel_id", dtype=DataType.VARCHAR, is_primary=False, max_length=64),
        FieldSchema(name="author_id", dtype=DataType.VARCHAR, is_primary=False, max_length=64),
        FieldSchema(name="embedding", dtype=DataType.FLOAT_VECTOR, dim=384),
        FieldSchema(name="mood_embedding", dtype=DataType.FLOAT_VECTOR, dim=768),  # Add new column
    ],
    description="Discord messages with embeddings"
)

# Create the new collection
new_collection = Collection(name="statisticsbot_new", schema=new_schema)

# Retrieve data from the old collection
old_collection = Collection(name="statisticsbot")
old_collection.load()

# Define query parameters
batch_size = 10000  # Number of records per batch
output_fields=["id","guild_id","channel_id","author_id", "embedding"]
expr = ""  # Query expression to retrieve all records
old_data = []

print(f"Total records in old collection: {old_collection.num_entities}")

# Use query_iterator to fetch data in batches
iterator = old_collection.query_iterator(
    expr=expr, output_fields=output_fields, batch_size=batch_size
)

try:
    while True:
        # Use .next() to get the next batch
        batch = iterator.next()
        if len(batch) == 0:
            break   
        old_data.extend(batch)
        print(f"Retrieved batch of {len(batch)} records.")
        # Process or save the batch as needed
except StopIteration:
    print("All records have been fetched.")
        
print(f"Total records retrieved: {len(old_data)}")

# # Insert data into the new collection
# new_data = [
#     [row["id"] for row in old_data],  # Existing IDs
#     [row["guild_id"] for row in old_data],  # Existing IDs
#     [row["channel_id"] for row in old_data],  # Existing IDs
#     [row["author_id"] for row in old_data],  # Existing IDs
#     [row["embedding"] for row in old_data],  # Existing embeddings
#     [[0.0] * 384 for _ in old_data]  # Placeholder mood embeddings (adjust as needed)
# ]
# new_collection.insert(new_data)


for i in range(0, len(old_data), batch_size):
    batch = old_data[i:i + batch_size]

    # Prepare the batch data
    new_data = [
        [row["id"] for row in batch],  # Existing IDs
        [row["guild_id"] for row in batch],  # Guild IDs
        [row["channel_id"] for row in batch],  # Channel IDs
        [row["author_id"] for row in batch],  # Author IDs
        [row["embedding"] for row in batch],  # Existing embeddings
        [[0.0] * 768 for _ in batch]  # Placeholder mood embeddings (adjust size if needed)
    ]
    
    # Insert the batch
    new_collection.insert(new_data)

# # Optional: Drop the old collection if no longer needed
# # old_collection.drop()
