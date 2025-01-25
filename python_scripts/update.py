import os
import json
from pymilvus import connections, Collection

# Constants
MILVUS_HOST = "localhost"
MILVUS_PORT = "19530"
COLLECTION_NAME = "statisticsbot"
IMPORT_DIR = "/home/stollenaar/Documents/milvus_import_chunks_slurped"

# Connect to Milvus
connections.connect(
    "default",
    host=MILVUS_HOST,
    port=MILVUS_PORT,
    grpc_options={
        "grpc.max_send_message_length": 100 * 1024 * 1024,  # 100 MB
        "grpc.max_receive_message_length": 100 * 1024 * 1024,  # 100 MB
    }
)

# Load the collection
collection = Collection(name=COLLECTION_NAME)


def batch_upsert(collection: Collection, updated_data: list, batch_size: int = 1000):
    """
    Perform upsert operation in batches.

    :param collection: The Milvus collection object.
    :param updated_data: The data to upsert, structured as a list of lists (columns of data).
    :param batch_size: Number of records per batch.
    """
    num_records = len(updated_data[0])  # Assuming all columns have the same number of records
    for i in range(0, num_records, batch_size):
        try:
            batch = [column[i:i + batch_size] for column in updated_data]  # Slice each column
            collection.upsert(batch)
            print(f"Upserted batch {i // batch_size + 1} with {len(batch[0])} records.")
        except Exception as e:
            print(f"Error processing batch: {e}")


# Function to update records
def update_records_from_json():
    # Get all JSON files from the directory
    json_files = [f for f in os.listdir(IMPORT_DIR) if f.endswith('.json')]

    for json_file in json_files:
        file_path = os.path.join(IMPORT_DIR, json_file)

        with open(file_path, 'r') as file:
            try:
                # Load JSON data
                records = json.load(file)

                             # Extract IDs and mood_embeddings from the JSON file
                ids = [str(record["id"]) for record in records]  # Ensure IDs are strings
                embedding = [record["embeddings"] for record in records]  # Use "embedding" for mood_embedding

                # Fetch existing records for the given IDs
                id_string = [f'"{str(id)}"' for id in ids]
                query = f'id in [{",".join(id_string)}]'
                #38,40,37
                batch_size = 600  # Adjust the batch size
                iterator = collection.query_iterator(expr=query, output_fields=["id", "guild_id", "channel_id", "author_id", "mood_embedding"], batch_size=batch_size)
                existing_records = []
                try:
                    while True:
                        batch = iterator.next()
                        if not batch:
                            break
                        existing_records.extend(batch)
                except StopIteration:
                    print("All records fetched.")

                # Prepare updated data
                updated_data = [
                    [row["id"] for row in existing_records],  # Keep IDs unchanged
                    [row["guild_id"] for row in existing_records],  # Keep guild_ids unchanged
                    [row["channel_id"] for row in existing_records],  # Keep channel_ids unchanged
                    [row["author_id"] for row in existing_records],  # Keep author_ids unchanged
                    embedding,  # Keep embeddings unchanged
                    [row["mood_embedding"] for row in existing_records],  # Keep embeddings unchanged
                ]

                # Update the Milvus collection
                batch_upsert(collection, updated_data, batch_size)
                print(f"Updated records from {json_file}")
            except Exception as e:
                print(f"Error processing {json_file}: {e}")

# Run the updater
if __name__ == "__main__":
    update_records_from_json()
