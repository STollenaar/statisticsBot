from sklearn.cluster import DBSCAN
from sklearn.feature_extraction.text import TfidfVectorizer
from sklearn.metrics.pairwise import cosine_similarity
from sklearn.preprocessing import normalize
from typing import List
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
    topN: int                       # The number of top terms to include in the title.

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

def get_topic_title(messages: List[str], top_n: int = 3) -> str:
   # Convert the messages to a TF-IDF matrix
    vectorizer = TfidfVectorizer(stop_words='english', max_features=50)  # Focus on most relevant terms
    tfidf_matrix = vectorizer.fit_transform(messages)
    
    # Get feature names (terms)
    feature_names = vectorizer.get_feature_names_out()
    
    # Sum the TF-IDF scores for each term across all messages in the cluster
    term_scores = tfidf_matrix.sum(axis=0).A1  # Summing along the documents
    
    # Get the indices of the top-n terms
    top_indices = term_scores.argsort()[-top_n:][::-1]
    
    # Combine the top terms into a topic title
    topic_title = ", ".join(feature_names[i] for i in top_indices)
    
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

    # Helper function for chunking and summarizing large texts
    def chunk_and_summarize(text: str, chunk_size: int = 1024) -> str:
        words = text.split()
        # Split the text into chunks
        chunks = [" ".join(words[i:i + chunk_size]) for i in range(0, len(words), chunk_size)]
        summaries = []
        for chunk in chunks:
            try:
                chunk_summary = summarizer(chunk, max_length=100, min_length=25, do_sample=False)
                summaries.append(chunk_summary[0]['summary_text'])
            except Exception as e:
                summaries.append(f"Error summarizing chunk: {str(e)}")
        # Combine summaries into a final summary
        combined_text = " ".join(summaries)
        try:
            final_summary = summarizer(combined_text, max_length=150, min_length=50, do_sample=False)
            return final_summary[0]['summary_text']
        except Exception as e:
            return f"Error combining summaries: {str(e)}"

    # Step 3: Generate summaries and topic titles for each group
    summaries = {}
    for label, group in grouped_messages.items():
        # Generate the topic title based on the group of messages
        topic_title = get_topic_title(group, request.topN)

        # Combine the messages for summarization
        combined_text = " ".join(group)
        try:
            summary = chunk_and_summarize(combined_text)
            summaries[topic_title] = summary
        except Exception as e:
            summaries[topic_title] = f"Error summarizing cluster {label}: {str(e)}"

    return {"summaries": summaries}
