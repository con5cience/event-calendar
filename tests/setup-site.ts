import { configureSite } from "../web/site";
import denver from "./contracts/site.json";
configureSite(denver as Parameters<typeof configureSite>[0]);
