from pathlib import Path

import pytest

from app.registry.skill_registry import SkillRegistry


def _write_skill_registry_fixture(
    tmp_path: Path,
    *,
    skill_dir_name: str = "refund-preview",
    frontmatter: str,
    body: str = "# refund-preview\n\n用于测试。\n",
    registry_skill_path: str | None = None,
    use_prompt_template: bool = False,
) -> Path:
    registry_path = tmp_path / "app" / "skills" / "registry.yaml"
    skill_dir = tmp_path / "app" / "skills" / skill_dir_name
    skill_dir.mkdir(parents=True, exist_ok=True)
    (skill_dir / "SKILL.md").write_text(f"---\n{frontmatter}\n---\n\n{body}", encoding="utf-8")

    skill_ref = registry_skill_path or f"skills/{skill_dir_name}/SKILL.md"
    skill_path_key = "prompt_template" if use_prompt_template else "skill_path"
    registry_path.write_text(
        "\n".join(
            [
                "skills:",
                "  refund.preview:",
                "    version: v1",
                "    domain: refund",
                "    agent_type: refund",
                f"    {skill_path_key}: {skill_ref}",
                "    provider: order",
                "    tools: [preview_refund_order]",
                "    termination_rule: got_preview_or_fail",
                "    fallback_policy: handoff",
            ]
        ),
        encoding="utf-8",
    )
    return registry_path


def test_skill_registry_loads_full_skill_markdown_for_runtime():
    registry = SkillRegistry.from_config(Path("app/skills/registry.yaml"))

    markdown = registry.load_skill_markdown("refund.preview")

    assert markdown.startswith("---")
    assert "allowed-tools: preview_refund_order" in markdown
    assert "目标：确认指定订单当前是否可退款" in markdown


def test_skill_registry_rejects_legacy_prompt_template_config(tmp_path: Path):
    registry_path = _write_skill_registry_fixture(
        tmp_path,
        frontmatter=(
            "name: refund-preview\n"
            "description: 用于预览订单退款资格。\n"
            "allowed-tools: preview_refund_order\n"
            "metadata:\n"
            "  domain: refund\n"
            "  skill_id: refund.preview\n"
        ),
        use_prompt_template=True,
    )

    with pytest.raises(ValueError, match="skill_path is required"):
        SkillRegistry.from_config(registry_path)


def test_skill_registry_rejects_non_string_allowed_tools(tmp_path: Path):
    registry_path = _write_skill_registry_fixture(
        tmp_path,
        frontmatter=(
            "name: refund-preview\n"
            "description: 用于预览订单退款资格。\n"
            "allowed-tools:\n"
            "  - preview_refund_order\n"
            "metadata:\n"
            "  domain: refund\n"
            "  skill_id: refund.preview\n"
        ),
    )

    with pytest.raises(ValueError, match="allowed-tools must be a space-delimited string"):
        SkillRegistry.from_config(registry_path)


def test_skill_registry_rejects_non_string_metadata_values(tmp_path: Path):
    registry_path = _write_skill_registry_fixture(
        tmp_path,
        frontmatter=(
            "name: refund-preview\n"
            "description: 用于预览订单退款资格。\n"
            "allowed-tools: preview_refund_order\n"
            "metadata:\n"
            "  domain: refund\n"
            "  priority: 1\n"
        ),
    )

    with pytest.raises(ValueError, match="skill metadata must be a string-to-string mapping"):
        SkillRegistry.from_config(registry_path)


def test_skill_registry_rejects_unknown_frontmatter_keys(tmp_path: Path):
    registry_path = _write_skill_registry_fixture(
        tmp_path,
        frontmatter=(
            "name: refund-preview\n"
            "description: 用于预览订单退款资格。\n"
            "allowed-tools: preview_refund_order\n"
            "foo: bar\n"
            "metadata:\n"
            "  domain: refund\n"
            "  skill_id: refund.preview\n"
        ),
    )

    with pytest.raises(ValueError, match="unsupported frontmatter keys"):
        SkillRegistry.from_config(registry_path)
