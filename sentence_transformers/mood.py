from typing import List

import numpy as np
from fastapi import APIRouter
from pydantic import BaseModel
from sklearn.cluster import HDBSCAN
from sklearn.metrics.pairwise import cosine_similarity
from sklearn.preprocessing import normalize
from transformers import pipeline

router = APIRouter()


class ClusterMood(BaseModel):
    mood: str
    messages: List[str]


class MoodResponse(BaseModel):
    mood: List[ClusterMood]


# Define the structure of the incoming request
class MoodBody(BaseModel):
    vector: List[float]  # Each vector for a message
    message: str  # Corresponding message


class MoodRequest(BaseModel):
    messages: List[MoodBody]  # List of vectors and messages to cluster
    eps: float  # Maximum distance between two samples for them to be considered as in the same neighborhood
    minSamples: int  # The number of samples in a neighborhood for a point to be considered as a core point
    topN: int  # The number of top terms to include in the title.


# Function to dynamically cluster messages using HDBSCAN
def cluster_messages(vectors: List[List[float]], min_samples: int) -> List[int]:
    # Normalize vectors for cosine similarity
    normalized_vectors = normalize(vectors)

    # Compute cosine similarity matrix
    similarity_matrix = cosine_similarity(normalized_vectors)

    # Convert similarity to a precomputed distance matrix
    distance_matrix = 1 - similarity_matrix
    distance_matrix[distance_matrix < 0] = 0
    print(np.min(distance_matrix), np.max(distance_matrix))
    # Perform HDBSCAN clustering
    clustering = HDBSCAN(
        min_samples=min_samples,
        metric="euclidean",  # Try Euclidean or other metrics
    )
    labels = clustering.fit_predict(vectors)
    return labels


@router.post("/mood")
async def mood(request: MoodRequest) -> MoodResponse:
    vectors = [item.vector for item in request.messages]
    messages = [item.message for item in request.messages]

    # Step 1: Cluster messages based on vectors using HDBSCAN
    labels = cluster_messages(vectors, request.minSamples)

    classifier = pipeline(
        "text-classification",
        model="j-hartmann/emotion-english-distilroberta-base",
        top_k=None,
    )

    cluster_moods = []
    for cluster in set(labels):  # For each unique cluster
        if cluster == -1:  # Skip noise
            continue

        # Get indices of messages in the current cluster
        cluster_indices = [i for i, label in enumerate(labels) if label == cluster]

        # Validate indices
        if any(i >= len(messages) for i in cluster_indices):
            raise ValueError("Cluster indices exceed the length of messages list")

        # Extract messages for the current cluster
        clustered_messages = [messages[i] for i in cluster_indices]

        cluster_scores = []
        for message in clustered_messages:
            scores = classifier(message)[0]
            mood_score = max(scores, key=lambda x: x["score"])
            cluster_scores.append(mood_score["label"])

        dominant_mood = max(set(cluster_scores), key=cluster_scores.count)
        cluster_moods.append(
            ClusterMood(mood=dominant_mood, messages=clustered_messages)
        )

    return MoodResponse(mood=cluster_moods)
