import os
import json
from pymilvus import connections, Collection

# Constants
MILVUS_HOST = "localhost"
MILVUS_PORT = "19530"
COLLECTION_NAME = "statisticsbot"
IMPORT_DIR = "/home/stollenaar/Documents/milvus_import_chunks_slurped"

# Connect to Milvus
connections.connect("default", host=MILVUS_HOST, port=MILVUS_PORT)

# Load the collection
collection = Collection(name=COLLECTION_NAME)

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
                mood_embedding = [record["embeddings"] for record in records]  # Use "embedding" for mood_embedding

                # Fetch existing records for the given IDs
                id_string = [f'"{str(id)}"' for id in ids]
                query = f'id in [{",".join(id_string)}]'
                existing_records = collection.query(expr=query, output_fields=["id", "guild_id", "channel_id", "author_id", "embedding"])

                # Prepare updated data
                updated_data = [
                    [row["id"] for row in existing_records],  # Keep IDs unchanged
                    [row["guild_id"] for row in existing_records],  # Keep guild_ids unchanged
                    [row["channel_id"] for row in existing_records],  # Keep channel_ids unchanged
                    [row["author_id"] for row in existing_records],  # Keep author_ids unchanged
                    [row["embedding"] for row in existing_records],  # Keep embeddings unchanged
                    mood_embedding  # Update mood_embeddings
                ]

                # Update the Milvus collection
                collection.upsert(updated_data)
                print(f"Updated records from {json_file}")
            except Exception as e:
                print(f"Error processing {json_file}: {e}")

# Run the updater
if __name__ == "__main__":
    update_records_from_json()
