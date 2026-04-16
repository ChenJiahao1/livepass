from pathlib import Path


def test_agents_readme_documents_thread_api():
    readme = Path("README.md").read_text(encoding="utf-8")

    assert "/agent/threads" in readme
    assert "/agent/chat" not in readme
    assert "Thread / Message / Run" in readme
