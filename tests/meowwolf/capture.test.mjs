import { test } from "node:test";
import assert from "node:assert/strict";
import {
  listing,
  detail,
  validateSnapshot,
  documentProps,
} from "./capture.mjs";

test("reads versioned Flight JSON without executing scripts", () => {
  const data = {
    children: [
      {
        seller: {
          id: "017a7f54-e443-a261-3c55-46ef4d921efb",
          timezone: "America/Denver",
        },
        siteSlug: "denver/events",
      },
      { eventsData: [], containsMultipleCategories: true },
    ],
  };
  const stream = "7:" + JSON.stringify(data) + "\n";
  const html =
    "<script>self.__next_f.push(" + JSON.stringify([1, stream]) + ")</script>";
  assert.deepEqual(documentProps(html).events.events, []);
  assert.throws(() =>
    documentProps("<script>throw new Error('Do not execute')</script>"),
  );
  assert.throws(() => documentProps(html.replace("eventsData", "unknownData")));
});

test("new detail envelope uses an associated-event object, not an array", () => {
  const event = {
    id: "example",
    sellerId: "017a7f54-e443-a261-3c55-46ef4d921efb",
  };
  const node = {
    event,
    eventVenue: { id: "room" },
    associatedEvents: { addOns: null, dayPass: null },
  };
  const stream = "8:" + JSON.stringify(node) + "\n";
  const html =
    "<script>self.__next_f.push(" + JSON.stringify([1, stream]) + ")</script>";
  assert.deepEqual(documentProps(html).events.events, [event]);
});

const seller = {
  id: "017a7f54-e443-a261-3c55-46ef4d921efb",
  timezone: "America/Denver",
};

test("unnumbered stylesheet hints are not event models", () => {
  const data = {
    children: [
      { seller, siteSlug: "denver/events" },
      { eventsData: [], containsMultipleCategories: true },
    ],
  };
  const stream =
    ':HL["/style.css","style"]\n2:X\n7:' + JSON.stringify(data) + "\n";
  const html =
    "<script>self.__next_f.push(" + JSON.stringify([1, stream]) + ")</script>";
  assert.deepEqual(documentProps(html).events.events, []);
});

test("length-framed descriptions cannot hide or inject the following event model", () => {
  const event = { id: "example", sellerId: seller.id };
  const model = {
    event,
    eventVenue: { id: "room" },
    associatedEvents: { addOns: null, dayPass: null },
  };
  const description = 'Music é\n8:{"event":"fake"}';
  const stream =
    "34:T" +
    Buffer.byteLength(description).toString(16) +
    "," +
    description +
    "8:" +
    JSON.stringify(model) +
    "\n";
  const html =
    "<script>self.__next_f.push(" + JSON.stringify([1, stream]) + ")</script>";
  assert.deepEqual(documentProps(html).events.events, [event]);
  assert.throws(() => documentProps(html.replace("34:T", "34:Tffff")));
});
const row = {
  id: "019f1aa0-d978-0ee7-4153-a9518ceb4d9a__be822f61-3e04-0559-7a43-ca5b19313b79",
  title: "Fixture",
  startDateTime: "2026-09-13T00:00:00Z",
  url: "fixture",
  text: {
    date: "Sep 12th Doors @ 6:00 PM",
    banner: null,
    supportingActs: null,
  },
  tags: ["all ages"],
};
const props = () => ({
  isEvents: true,
  events: { seller, events: [structuredClone(row)] },
});
test("listing selects publication fields and verifies rendered links", () => {
  assert.deepEqual(listing(props(), ["/events/denver/fixture/"]), [row]);
  assert.deepEqual(
    listing({ ...props(), events: { seller, events: [] } }, []),
    [],
  );
});
for (const kind of [
  "scope",
  "missing",
  "identity",
  "links",
  "duplicate",
  "cap",
]) {
  test(`listing rejects ${kind}`, () => {
    const p = props();
    let links = ["/events/denver/fixture/"];
    if (kind === "scope") p.events.seller = { ...seller, id: "other" };
    if (kind === "missing") delete p.events.events;
    if (kind === "identity") p.events.events[0].id = "bad";
    if (kind === "links") links = [];
    if (kind === "duplicate") p.events.events.push(row);
    if (kind === "cap") p.events.events = Array(501).fill(row);
    assert.throws(() => listing(p, links));
  });
}
test("detail selects no price/capacity data and recognizes unrelated redirects", () => {
  const event = {
    id: row.id.split("__")[0],
    sellerId: seller.id,
    venueId: "room",
    title: "Fixture",
    meta: [
      { metakey: "eventAge", value: "All Ages" },
      { metakey: "price", value: "50" },
    ],
    timeslots: [
      { id: row.id.split("__")[1], startTime: row.startDateTime, capacity: 50 },
    ],
  };
  const d = detail(
    { isEvents: true, isDetails: true, seller, events: { events: [event] } },
    row,
  );
  assert.deepEqual(d.meta, [{ metakey: "eventAge", value: "All Ages" }]);
  assert(!JSON.stringify(d).includes("capacity"));
  assert.deepEqual(detail({ __N_REDIRECT: "/events/denver/" }, row), {
    error: "unrelated redirect",
  });
  assert.throws(() => detail({}, row));
  assert.equal(
    detail(
      {
        isEvents: true,
        isDetails: true,
        seller,
        events: { events: [{ ...event, id: "other" }] },
      },
      row,
    ).id,
    "other",
  );
  for (const changed of [{ id: "" }, { meta: null }, { timeslots: null }])
    assert.throws(() =>
      detail(
        {
          isEvents: true,
          isDetails: true,
          seller,
          events: { events: [{ ...event, ...changed }] },
        },
        row,
      ),
    );
});
test("paired captures must match", () => {
  assert.equal(validateSnapshot([row], [structuredClone(row)]).total, 1);
  assert.throws(() => validateSnapshot([row], []));
});
