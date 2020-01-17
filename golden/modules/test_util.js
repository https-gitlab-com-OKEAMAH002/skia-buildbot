/** @module golden/test_util
 * @description
 *
 * <p>
 *  A general set of useful functions for tests and demos,
 *  e.g. reducing boilerplate.
 * </p>
 */

import { UNMATCHED } from 'fetch-mock';

/** expectNoUnmatchedCalls asserts that there were no
 *  unexpected (unmatched) calls to fetchMock.
 */
export function expectNoUnmatchedCalls(fetchMock) {
    let calls = fetchMock.calls(UNMATCHED, 'GET');
    expect(calls.length, 'no unmatched (unexpected) GETs').to.equal(0);
    if (calls.length) {
      console.warn('unmatched GETS', calls);
    }
    calls = fetchMock.calls(UNMATCHED, 'POST');
    expect(calls.length, 'no unmatched (unexpected) POSTs').to.equal(0);
    if (calls.length) {
      console.warn('unmatched POSTS', calls);
    }
}

/**
 * Returns a promise that will resolve when the given event is caught at the
 * document's body element, or reject if the event isn't caught within the given
 * amount of time.
 *
 * Set timeoutMillis = 0 to skip call to setTimeout(). This is necessary on
 * tests that simulate the passing of time with sinon.useFakeTimers(), which
 * could trigger the timeout before the promise has a chance to catch the event.
 *
 * Sample usage:
 *
 *   // Code under test.
 *   function doSomethingThatTriggersCustomEvent() {
 *     ...
 *     this.dispatchEvent(
 *         new CustomEvent('my-event', {detail: {foo: 'bar'}, bubbles: true});
 *   }
 *
 *   // Test.
 *   it('should trigger a custom event', async () => {
 *     const myEvent = eventPromise('my-event');
 *     doSomethingThatTriggersCustomEvent();
 *     const ev = await myEvent;
 *     expect(ev.detail.foo).to.equal('bar');
 *   });
 *
 * @param event {string} Name of event to catch.
 * @param timeoutMillis {number} How long to wait for the event before rejecting
 *     the returned promise.
 * @return {Promise} A promise that will resolve to the caught event.
 */
export function eventPromise(event, timeoutMillis = 5000) {
  const eventCaughtCallback = (resolve, _, e) => resolve(e);
  const timeoutCallback =
      (_, reject) =>
          reject(new Error(`timed out after ${timeoutMillis} ms ` +
                           `while waiting to catch event "${event}"`));
  return buildEventPromise(
      event, timeoutMillis, eventCaughtCallback, timeoutCallback);
}

/**
 * Helper function to construct promises based on DOM events.
 *
 * @param event {string} Name of event to add a listener for at document.body.
 * @param timeoutMillis {number} Milliseconds to wait before timing out.
 * @param eventCaughtCallback {Function} Called when the event is caught with
 *     parameters (resolve, reject, event), where resolve and reject are the
 *     functions passed to the promise's executor function, and event is the
 *     Event object that was caught.
 * @param timeoutCallback Called after timeoutMillis if no event is caught, with
 *     arguments (resolve, reject) as passed to eventCaughtCallback.
 * @return {Promise} A promise that will resolve or reject based exclusively on
 *     what the callback functions do with the resolve and reject parameters.
 */
function buildEventPromise(
    event, timeoutMillis, eventCaughtCallback, timeoutCallback) {
  // The executor function passed as a constructor argument to the Promise
  // object is executed immediately. This guarantees that the event handler
  // is added to document.body before returning.
  return new Promise((resolve, reject) => {
    let timeout;
    const handler = (e) => {
      document.body.removeEventListener(event, handler);
      clearTimeout(timeout);
      eventCaughtCallback(resolve, reject, e);
    };
    // Skip setTimeout() call with timeoutMillis = 0. Useful when faking time in
    // tests with sinon.useFakeTimers(). See
    // https://sinonjs.org/releases/v7.5.0/fake-timers/.
    if (timeoutMillis !== 0) {
      timeout = setTimeout(() => {
        document.body.removeEventListener(event, handler);
        timeoutCallback(resolve, reject);
      }, timeoutMillis);
    }
    document.body.addEventListener(event, handler);
  });
}

/** expectQueryStringToEqual asserts that there the given string
 *  exactly matches the current query string of the url bar.
 *  For example, expectQueryStringToEqual('?foo=bar&foo=orange');
 */
export function expectQueryStringToEqual(expected) {
  expect(window.location.search).to.equal(expected);
}
