from pymilvus import connections, Collection
import json
import numpy as np

# Connect to Milvus
connections.connect("default", host="localhost", port="19530")

# Retrieve data from the old collection
old_collection = Collection(name="statisticsbot_old")
old_collection.load()

# Define query parameters
batch_size = 10000  # Number of records per batch
output_fields=["id","guild_id","channel_id","author_id", "embedding"]
output_file = "export.json"
expr = ""  # Query expression to retrieve all records

# Convert data to JSON-serializable types
def make_json_serializable(data):
    if isinstance(data, np.ndarray):
        return data.tolist()  # Convert numpy arrays to lists
    if isinstance(data, (np.float32, np.float64, np.int32, np.int64)):
        return data.item()  # Convert numpy scalars to native Python types
    if isinstance(data, list):
        return [make_json_serializable(item) for item in data]  # Recursively process lists
    return data  # Return as-is for serializable types

# Initialize an empty list to store all data
all_data = []

# Use query_iterator to fetch and process data
iterator = old_collection.query_iterator(expr=expr, output_fields=output_fields, batch_size=batch_size)

try:
    while True:
        # Use .next() to get the next batch
        batch = iterator.next()
        if not batch :
            break

        # Process the batch
        for row in batch:
            processed_row = {
                "id": row["id"],
                "guild_id": row["guild_id"],
                "channel_id": row["channel_id"],
                "author_id": row["author_id"],
                "embedding": make_json_serializable(row["embedding"]),  # Ensure embedding is serializable
            }
            all_data.append(processed_row)
        print(f"Retrieved batch of {len(batch)} records.")

except StopIteration:
    print("All records have been fetched.")

# Write all data to a JSON file
with open(output_file, "w") as f:
    json.dump(all_data, f, indent=4)

print(f"Data successfully exported to {output_file}")