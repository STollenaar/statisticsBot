from fastapi import FastAPI
from pydantic import BaseModel
from summarize import router
from typing import Dict
from sentence_transformers import SentenceTransformer

# Initialize the app and model
app = FastAPI()
model = SentenceTransformer('all-MiniLM-L6-v2')  # Use any SentenceTransformers model
mood_model = SentenceTransformer('j-hartmann/emotion-english-distilroberta-base')

app.include_router(router)

# Request and response models
class TextRequest(BaseModel):
    text: str

class EmbeddingResponse(BaseModel):
    embedding: list
    moodEmbedding: list

@app.post("/embed", response_model=EmbeddingResponse)
async def embed(request: TextRequest):
    embedding = model.encode(request.text).tolist()
    mood_embedding = mood_model.encode(request.text).tolist()
    return {"embedding": embedding, "moodEmbedding":mood_embedding}

# Health check endpoint
@app.get("/healthz")
async def health_check() -> Dict[str, str]:
    # Return a simple message indicating that the service is healthy
    return {"status": "ok"}
