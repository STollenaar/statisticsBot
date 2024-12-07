from fastapi import FastAPI
from pydantic import BaseModel
from sentence_transformers import SentenceTransformer

# Initialize the app and model
app = FastAPI()
model = SentenceTransformer('all-MiniLM-L6-v2')  # Use any SentenceTransformers model

# Request and response models
class TextRequest(BaseModel):
    text: str

class EmbeddingResponse(BaseModel):
    embedding: list

@app.post("/embed", response_model=EmbeddingResponse)
async def embed(request: TextRequest):
    embedding = model.encode(request.text).tolist()
    return {"embedding": embedding}
