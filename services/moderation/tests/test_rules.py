from pathlib import Path

import pytest

from moderation.config import DEFAULT_RULES
from moderation.rules.engine import Action, RuleEngine, tokens


@pytest.fixture(scope="module")
def engine() -> RuleEngine:
    return RuleEngine.from_file(DEFAULT_RULES)


@pytest.mark.parametrize(
    ("text", "action", "reason"),
    [
        ("Trop beau ce coucher de soleil sur Abidjan 🔥", Action.ALLOW, None),
        ("", Action.ALLOW, None),
        ("Sale Nègre", Action.BLOCK, "hate"),
        ("je vais te TUER demain", Action.BLOCK, "violence"),
        ("Appelle le 07 07 07 07 07 pour ton dépôt Orange Money", Action.BLOCK, "fraud"),
        ("+221 77 123 45 67 envoie ton code", Action.BLOCK, "fraud"),
        ("S@L0PE", Action.REVIEW, "harassment"),
        ("t'es un connnnnard", Action.REVIEW, "harassment"),
        ("c o n n a r d", Action.REVIEW, "harassment"),
        ("Mon numéro : 05 99 88 77 66", Action.REVIEW, "spam"),
        ("www.a.com www.b.net http://c.xyz", Action.REVIEW, "spam"),
        ("GROSSE PROMO SUR TOUS NOS PRODUITS AUJOURDHUI", Action.REVIEW, "spam"),
        ("nooooooooooooooon", Action.REVIEW, "spam"),
        ("achete achete achete achete achete achete", Action.REVIEW, "spam"),
    ],
)
def test_verdicts(engine: RuleEngine, text: str, action: Action, reason: str | None) -> None:
    verdict = engine.check(text)
    assert verdict.action is action
    assert verdict.reason == reason
    assert (verdict.matches == []) is (action is Action.ALLOW)


def test_words_inside_other_words_do_not_match(engine: RuleEngine) -> None:
    assert engine.check("computerpute").action is Action.ALLOW
    assert engine.check("Je suis débile... non, débilement heureux").action is Action.REVIEW
    assert engine.check("la reputation du quartier").action is Action.ALLOW


def test_tokens_normalize() -> None:
    assert tokens("Élève l'ÉCOLE, 3ncul3 !!") == ["eleve", "l", "ecole", "encule"]


def test_bad_rule_files_are_refused(tmp_path: Path) -> None:
    bad = tmp_path / "bad.yml"
    bad.write_text("block:\n  boredom:\n    - x\n", encoding="utf-8")
    with pytest.raises(ValueError, match="unknown reason"):
        RuleEngine.from_file(bad)
    bad.write_text("block:\n  hate:\n    - '!!!'\n", encoding="utf-8")
    with pytest.raises(ValueError, match="empty term"):
        RuleEngine.from_file(bad)
    bad.write_text("- just a list\n", encoding="utf-8")
    with pytest.raises(ValueError, match="not a mapping"):
        RuleEngine.from_file(bad)
    empty = tmp_path / "empty.yml"
    empty.write_text("", encoding="utf-8")
    assert RuleEngine.from_file(empty).check("connard").action is Action.ALLOW
