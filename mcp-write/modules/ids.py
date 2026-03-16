"""Детерминированные id для чанков и точек Qdrant."""
import hashlib


def deterministic_chunk_id(doc_id: str, version_id: str, section_path: str, normalized_text: str) -> str:
    h = hashlib.sha256(f"{doc_id}:{version_id}:{section_path}:{normalized_text}".encode()).hexdigest()
    return h[:32]


def chunk_id_to_point_id(chunk_id: str) -> int:
    return int(hashlib.sha256(chunk_id.encode()).hexdigest()[:15], 16) & 0x7FFFFFFFFFFFFFFF


def point_id_for_doc_collection(doc_id: str, collection_name: str) -> int:
    """Один point на документ в коллекции — при повторном инжесте перезаписываем, дубли не копим."""
    return chunk_id_to_point_id(f"{doc_id}:{collection_name}")


def deterministic_edge_id(from_id: str, to_id: str, relation: str) -> str:
    h = hashlib.sha256(f"{from_id}:{to_id}:{relation}".encode()).hexdigest()
    return h[:32]
