from sklearn.cluster import DBSCAN
from sklearn.feature_extraction.text import TfidfVectorizer
from sklearn.metrics.pairwise import cosine_similarity
from sklearn.preprocessing import normalize
from pydantic import BaseModel
from typing import List, Dict
from transformers import pipeline
from fastapi import APIRouter

router = APIRouter()

# Initialize summarization pipeline
summarizer = pipeline("summarization")

# Define the structure of the incoming request
class SummaryBody(BaseModel):
    vector: List[float]  # Each vector for a message
    message: str         # Corresponding message

class SummaryRequest(BaseModel):
    messages: List[SummaryBody]  # List of vectors and messages to cluster
    eps: float                      # Maximum distance between two samples for them to be considered as in the same neighborhood
    minSamples: int                 # The number of samples in a neighborhood for a point to be considered as a core point

# Function to dynamically cluster messages using DBSCAN
def cluster_messages(vectors: List[List[float]], similarity_threshold: float, min_samples: int) -> List[int]:
    # DBSCAN clustering
    # # Normalize vectors to ensure they are unit vectors
    # normalized_vectors = normalize(vectors)
    
  # Normalize vectors for cosine similarity
    normalized_vectors = normalize(vectors)

    # Compute cosine similarity matrix
    similarity_matrix = cosine_similarity(normalized_vectors)
    # Convert similarity to a precomputed distance matrix
    distance_matrix = 1 - similarity_matrix
    distance_matrix[distance_matrix < 0] = 0

    # Perform DBSCAN with adjusted eps
    clustering = DBSCAN(eps=1 - similarity_threshold, min_samples=min_samples, metric="precomputed")
    labels = clustering.fit_predict(distance_matrix)
    return labels

# Function to extract topic title using TF-IDF

def get_topic_title(messages: List[str]) -> str:
    # Convert the messages to a TF-IDF matrix
    vectorizer = TfidfVectorizer(stop_words='english', max_features=10)  # Consider more terms for better context
    tfidf_matrix = vectorizer.fit_transform(messages)
    
    # Get feature names (terms)
    feature_names = vectorizer.get_feature_names_out()
    
    # Sum the TF-IDF scores for each term across all messages
    term_scores = tfidf_matrix.sum(axis=0).A1  # Summing along the documents
    max_index = term_scores.argmax()  # Index of the term with the highest score
    
    # The term with the highest TF-IDF score
    topic_title = feature_names[max_index]
    
    return topic_title

# API endpoint for clustering and summarization
@router.post("/summarize")
async def summarize(request: SummaryRequest) -> Dict[str, Dict[str, str]]:
    # Extract vectors and messages from the request
    vectors = [item.vector for item in request.messages]
    messages = [item.message for item in request.messages]

    # Step 1: Cluster messages based on vectors using DBSCAN
    labels = cluster_messages(vectors, request.eps, request.minSamples)

    # Step 2: Group messages by cluster
    grouped_messages = {label: [] for label in set(labels) if label != -1}  # Ignore noise (-1)
    for i, label in enumerate(labels):
        if label != -1:  # Skip noise points
            grouped_messages[label].append(messages[i])

    # Step 3: Generate summaries and topic titles for each group
    summaries = {}
    for label, group in grouped_messages.items():
        # Generate the topic title based on the group of messages
        topic_title = get_topic_title(group)

        # Combine the messages for summarization
        combined_text = " ".join(group)
        try:
            summary = summarizer(combined_text, max_length=100, min_length=25, do_sample=False)
            summaries[topic_title] = summary[0]['summary_text']
        except Exception as e:
            summaries[topic_title] = f"Error summarizing cluster {label}: {str(e)}"

    return {"summaries": summaries}