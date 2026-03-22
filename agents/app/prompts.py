"""Prompt loading helpers."""

from pathlib import Path
from typing import Any

from jinja2 import Environment, FileSystemLoader, StrictUndefined


PROMPT_SEARCH_PATHS = (
    Path(__file__).resolve().parent.parent / "prompts",
    Path(__file__).resolve().parent / "prompts",
)


class PromptRenderer:
    """Render Jinja-based prompt templates from the prompt directories."""

    def __init__(self, search_paths: tuple[Path, ...] = PROMPT_SEARCH_PATHS) -> None:
        loader = FileSystemLoader([str(path) for path in search_paths if path.exists()])
        self._env = Environment(
            loader=loader,
            autoescape=False,
            trim_blocks=True,
            lstrip_blocks=True,
            undefined=StrictUndefined,
        )

    def render(self, template_name: str, **context: Any) -> str:
        return self._env.get_template(template_name).render(**context).strip()
