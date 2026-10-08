"""Rule engine. Verdicts are explainable: each one lists the rules that fired."""

import re
import unicodedata
from dataclasses import dataclass, field
from enum import StrEnum
from pathlib import Path

import yaml

REASONS = ("spam", "nudity", "violence", "harassment", "hate", "fraud", "other")


class Action(StrEnum):
    ALLOW = "allow"
    REVIEW = "review"
    BLOCK = "block"


@dataclass(frozen=True)
class Verdict:
    action: Action
    reason: str | None = None
    matches: list[str] = field(default_factory=list)


ALLOW = Verdict(Action.ALLOW)

_LEET = str.maketrans({"0": "o", "1": "i", "3": "e", "4": "a", "5": "s", "7": "t", "@": "a", "$": "s"})
_URL = re.compile(r"(https?://\S+|www\.\S+|\b[a-z0-9-]+\.(?:com|net|org|info|xyz|top|ci|sn|cm|fr|ly|me)\b)")
# Côte d'Ivoire (10 chiffres), Sénégal (9, mobiles en 7), Cameroun (9, mobiles en 6),
# avec ou sans indicatif et séparateurs.
_PHONE = re.compile(
    r"(?<!\d)(?:(?:\+|00)?225[\s.-]?)?0[1579](?:[\s.-]?\d{2}){4}(?!\d)"
    r"|(?<!\d)(?:(?:\+|00)?221[\s.-]?)?7[05678](?:[\s.-]?\d){7}(?!\d)"
    r"|(?<!\d)(?:(?:\+|00)?237[\s.-]?)?6(?:[\s.-]?\d){8}(?!\d)"
)
MAX_LINKS = 2
CAPS_MIN_LETTERS = 20
CAPS_RATIO = 0.7
MAX_REPEATED_TOKEN = 5
MAX_CHAR_RUN = 10


def _strip_accents(text: str) -> str:
    decomposed = unicodedata.normalize("NFKD", text)
    return "".join(c for c in decomposed if not unicodedata.combining(c))


def tokens(text: str) -> list[str]:
    """Normalized words: lowercase, no accents, leetspeak undone, letter runs collapsed."""
    flat = _strip_accents(text).lower().translate(_LEET)
    flat = re.sub(r"(.)\1+", r"\1", flat)
    words = re.findall(r"[a-z0-9']+", flat)
    out: list[str] = []
    for word in words:
        out.extend(part for part in word.replace("'", " ").split() if part)
    return out


def _joined_letters(words: list[str]) -> list[str]:
    """Rebuild words written letter by letter (« c o n n a r d »)."""
    out: list[str] = []
    run: list[str] = []
    for word in [*words, ""]:
        if len(word) == 1:
            run.append(word)
            continue
        if len(run) >= 3:
            out.append(re.sub(r"(.)\1+", r"\1", "".join(run)))
        run = []
    return out


def _contains(words: list[str], term: list[str]) -> bool:
    n = len(term)
    return any(words[i : i + n] == term for i in range(len(words) - n + 1))


@dataclass(frozen=True)
class _Term:
    words: list[str]
    reason: str
    label: str


class RuleEngine:
    def __init__(self, block: list[_Term], review: list[_Term], money: list[list[str]]) -> None:
        self._block = block
        self._review = review
        self._money = money

    @classmethod
    def from_file(cls, path: Path) -> "RuleEngine":
        doc = yaml.safe_load(path.read_text(encoding="utf-8")) or {}
        if not isinstance(doc, dict):
            raise ValueError(f"{path}: not a mapping")

        def terms(level: str) -> list[_Term]:
            section = doc.get(level) or {}
            out: list[_Term] = []
            for reason, entries in section.items():
                if reason not in REASONS:
                    raise ValueError(f"{path}: unknown reason {reason!r} in {level}")
                for entry in entries or []:
                    words = tokens(str(entry))
                    if not words:
                        raise ValueError(f"{path}: empty term in {level}.{reason}")
                    out.append(_Term(words, reason, f"{level}:{reason}:{' '.join(words)}"))
            return out

        money = [tokens(str(term)) for term in doc.get("money_terms") or []]
        return cls(terms("block"), terms("review"), [m for m in money if m])

    def check(self, text: str) -> Verdict:
        """Return the strongest verdict for text; block wins over review."""
        if not text.strip():
            return ALLOW
        words = tokens(text)
        candidates = words + _joined_letters(words)
        for term in self._block:
            if _contains(words, term.words) or (len(term.words) == 1 and term.words[0] in candidates):
                return Verdict(Action.BLOCK, term.reason, [term.label])

        phones = _PHONE.findall(text)
        if phones and any(_contains(words, money) for money in self._money):
            return Verdict(Action.BLOCK, "fraud", ["block:fraud:phone+money"])

        review: list[str] = []
        reason: str | None = None
        for term in self._review:
            if _contains(words, term.words) or (len(term.words) == 1 and term.words[0] in candidates):
                review.append(term.label)
                reason = reason or term.reason
        lowered = _strip_accents(text).lower()
        if len(_URL.findall(lowered)) > MAX_LINKS:
            review.append("review:spam:links")
            reason = reason or "spam"
        if phones:
            review.append("review:spam:phone")
            reason = reason or "spam"
        letters = [c for c in text if c.isalpha()]
        if len(letters) >= CAPS_MIN_LETTERS and sum(c.isupper() for c in letters) / len(letters) > CAPS_RATIO:
            review.append("review:spam:caps")
            reason = reason or "spam"
        if re.search(rf"(.)\1{{{MAX_CHAR_RUN},}}", text) or any(
            words.count(w) > MAX_REPEATED_TOKEN for w in set(words)
        ):
            review.append("review:spam:repetition")
            reason = reason or "spam"
        if review:
            return Verdict(Action.REVIEW, reason, review)
        return ALLOW
