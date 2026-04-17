"""Prompt loading helpers."""

from pathlib import Path
from typing import Any

from jinja2 import Environment, FileSystemLoader, StrictUndefined


PROMPT_SEARCH_PATH = Path(__file__).resolve().parents[2] / "prompts"


class PromptLoader:
    """Render Jinja-based role prompt templates."""

    def __init__(self, search_path: Path = PROMPT_SEARCH_PATH) -> None:
        self._env = Environment(
            loader=FileSystemLoader(str(search_path)),
            autoescape=False,
            trim_blocks=True,
            lstrip_blocks=True,
            undefined=StrictUndefined,
        )

    def render(self, role_name: str, **context: Any) -> str:
        return self._env.get_template(f"{role_name}.md").render(**context).strip()
