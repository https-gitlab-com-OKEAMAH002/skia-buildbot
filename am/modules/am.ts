// Functions used by more than one element.
import { diffDate } from 'common-sk/modules/human';
import { unsafeHTML } from 'lit-html/directives/unsafe-html';
import { TemplateResult, html, Part } from 'lit-html';
import { SilenceSk, State } from './silence-sk/silence-sk';
import { IncidentSk } from './incident-sk/incident-sk';
import { Note, Silence } from './json';

const linkRe = /(http[s]?:\/\/[^\s]*)/gm;

/**
 * Formats the text for a silence header.
 *
 * silence - The silence being displayed.
 */
export function displaySilence(silence: Silence | State): TemplateResult {
  const ret: string[] = [];
  Object.keys(silence.param_set!).forEach((key) => {
    if (key.startsWith('__')) {
      return;
    }
    ret.push(`${silence.param_set![key]!.join(', ')}`);
  });
  const fullName = ret.join(' ');
  let displayName = fullName;
  if (displayName.length > 33) {
    displayName = `${displayName.slice(0, 30)}...`;
  }
  if (!displayName.length) {
    displayName = '(*)';
  }
  return html`<span title="${fullName}">${displayName}</span>`;
}

/**
 * Returns the params.abbr to be appended to a string, if present.
 */
export function abbr(paramsAbbr: string): string {
  if (paramsAbbr) {
    return ` - ${paramsAbbr}`;
  }
  return '';
}

/**
 * Convert all URLs in a string into links in a lit-html TemplateResult.
 */
export function linkify(s: string): (part: Part)=> void {
  return unsafeHTML(s.replace(linkRe, '<a href="$&" rel=noopener target=_blank>$&</a>'));
}

/**
 * Templates notes to be displayed.
 */
export function displayNotes(notes: Note[], ele: SilenceSk|IncidentSk): TemplateResult[] {
  if (!notes) {
    return [];
  }
  return notes.map((note: Note, index: number) => html`<section class=note>
  <p class="note-text">${linkify(note.text)}</p>
  <div class=meta>
    <span class=author>${note.author}</span>
    <span class=date>${diffDate(note.ts * 1000)}</span>
    <delete-icon-sk title='Delete comment.' @click=${(e: Event) => ele.deleteNote(e, index)}></delete-icon-sk>
  </div>
</section>`);
}

const TIME_DELTAS = [
  { units: 'w', delta: 7 * 24 * 60 * 60 },
  { units: 'd', delta: 24 * 60 * 60 },
  { units: 'h', delta: 60 * 60 },
  { units: 'm', delta: 60 },
  { units: 's', delta: 1 },
];

/**
 * Returns the parsed duration in seconds.
 *
 * @param {string} d - The duration, e.g. "2h" or "4d".
 * @returns {number} The duration in seconds.
 *
 * TODO(jcgregorio) Move into common-sk/modules/human.js with tests.
 */
export function parseDuration(d: string): number {
  const units = d.slice(-1);
  const scalar = +d.slice(0, -1);
  for (let i = 0; i < TIME_DELTAS.length; i++) {
    const o = TIME_DELTAS[i];
    if (o.units === units) {
      return o.delta * scalar;
    }
  }
  return 0;
}

export function expiresIn(silence: Silence| State): string {
  if (silence.active) {
    return diffDate((silence.created + parseDuration(silence.duration)) * 1000);
  }
  return '';
}

/**
 * Returns the human.diffDate str for the duration from the current date
 * till the next day of the week at the specified time.
 *
 * @param {number} day - Possible values range from 0 to 6 for Sun to Sat.
 * @param {number} hour - Possible values range from 0 to 23 for the hour.
 */
export function getDurationTillNextDay(day: number, hour: number): string {
  const now = new Date();
  const y = now.getFullYear();
  const m = now.getMonth();
  let d = now.getDate();

  // Increment d till we reach the target day.
  let tmp = new Date();
  do {
    tmp = new Date(y, m, ++d);
  } while (tmp.getDay() !== day);
  const target = new Date(y, m, d, hour, 0, 0);

  return diffDate(target.toString());
}
