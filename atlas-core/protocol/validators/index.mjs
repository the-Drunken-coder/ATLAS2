var __getOwnPropNames = Object.getOwnPropertyNames;
var __commonJS = (cb, mod) => function __require() {
  return mod || (0, cb[__getOwnPropNames(cb)[0]])((mod = { exports: {} }).exports, mod), mod.exports;
};

// node_modules/.pnpm/ajv@8.20.0/node_modules/ajv/dist/runtime/ucs2length.js
var require_ucs2length = __commonJS({
  "node_modules/.pnpm/ajv@8.20.0/node_modules/ajv/dist/runtime/ucs2length.js"(exports) {
    "use strict";
    Object.defineProperty(exports, "__esModule", { value: true });
    function ucs2length(str) {
      const len = str.length;
      let length = 0;
      let pos = 0;
      let value;
      while (pos < len) {
        length++;
        value = str.charCodeAt(pos++);
        if (value >= 55296 && value <= 56319 && pos < len) {
          value = str.charCodeAt(pos);
          if ((value & 64512) === 56320)
            pos++;
        }
      }
      return length;
    }
    exports.default = ucs2length;
    ucs2length.code = 'require("ajv/dist/runtime/ucs2length").default';
  }
});

// node_modules/.pnpm/ajv-formats@3.0.1_ajv@8.20.0/node_modules/ajv-formats/dist/formats.js
var require_formats = __commonJS({
  "node_modules/.pnpm/ajv-formats@3.0.1_ajv@8.20.0/node_modules/ajv-formats/dist/formats.js"(exports) {
    "use strict";
    Object.defineProperty(exports, "__esModule", { value: true });
    exports.formatNames = exports.fastFormats = exports.fullFormats = void 0;
    function fmtDef(validate, compare) {
      return { validate, compare };
    }
    exports.fullFormats = {
      // date: http://tools.ietf.org/html/rfc3339#section-5.6
      date: fmtDef(date, compareDate),
      // date-time: http://tools.ietf.org/html/rfc3339#section-5.6
      time: fmtDef(getTime(true), compareTime),
      "date-time": fmtDef(getDateTime(true), compareDateTime),
      "iso-time": fmtDef(getTime(), compareIsoTime),
      "iso-date-time": fmtDef(getDateTime(), compareIsoDateTime),
      // duration: https://tools.ietf.org/html/rfc3339#appendix-A
      duration: /^P(?!$)((\d+Y)?(\d+M)?(\d+D)?(T(?=\d)(\d+H)?(\d+M)?(\d+S)?)?|(\d+W)?)$/,
      uri,
      "uri-reference": /^(?:[a-z][a-z0-9+\-.]*:)?(?:\/?\/(?:(?:[a-z0-9\-._~!$&'()*+,;=:]|%[0-9a-f]{2})*@)?(?:\[(?:(?:(?:(?:[0-9a-f]{1,4}:){6}|::(?:[0-9a-f]{1,4}:){5}|(?:[0-9a-f]{1,4})?::(?:[0-9a-f]{1,4}:){4}|(?:(?:[0-9a-f]{1,4}:){0,1}[0-9a-f]{1,4})?::(?:[0-9a-f]{1,4}:){3}|(?:(?:[0-9a-f]{1,4}:){0,2}[0-9a-f]{1,4})?::(?:[0-9a-f]{1,4}:){2}|(?:(?:[0-9a-f]{1,4}:){0,3}[0-9a-f]{1,4})?::[0-9a-f]{1,4}:|(?:(?:[0-9a-f]{1,4}:){0,4}[0-9a-f]{1,4})?::)(?:[0-9a-f]{1,4}:[0-9a-f]{1,4}|(?:(?:25[0-5]|2[0-4]\d|[01]?\d\d?)\.){3}(?:25[0-5]|2[0-4]\d|[01]?\d\d?))|(?:(?:[0-9a-f]{1,4}:){0,5}[0-9a-f]{1,4})?::[0-9a-f]{1,4}|(?:(?:[0-9a-f]{1,4}:){0,6}[0-9a-f]{1,4})?::)|[Vv][0-9a-f]+\.[a-z0-9\-._~!$&'()*+,;=:]+)\]|(?:(?:25[0-5]|2[0-4]\d|[01]?\d\d?)\.){3}(?:25[0-5]|2[0-4]\d|[01]?\d\d?)|(?:[a-z0-9\-._~!$&'"()*+,;=]|%[0-9a-f]{2})*)(?::\d*)?(?:\/(?:[a-z0-9\-._~!$&'"()*+,;=:@]|%[0-9a-f]{2})*)*|\/(?:(?:[a-z0-9\-._~!$&'"()*+,;=:@]|%[0-9a-f]{2})+(?:\/(?:[a-z0-9\-._~!$&'"()*+,;=:@]|%[0-9a-f]{2})*)*)?|(?:[a-z0-9\-._~!$&'"()*+,;=:@]|%[0-9a-f]{2})+(?:\/(?:[a-z0-9\-._~!$&'"()*+,;=:@]|%[0-9a-f]{2})*)*)?(?:\?(?:[a-z0-9\-._~!$&'"()*+,;=:@/?]|%[0-9a-f]{2})*)?(?:#(?:[a-z0-9\-._~!$&'"()*+,;=:@/?]|%[0-9a-f]{2})*)?$/i,
      // uri-template: https://tools.ietf.org/html/rfc6570
      "uri-template": /^(?:(?:[^\x00-\x20"'<>%\\^`{|}]|%[0-9a-f]{2})|\{[+#./;?&=,!@|]?(?:[a-z0-9_]|%[0-9a-f]{2})+(?::[1-9][0-9]{0,3}|\*)?(?:,(?:[a-z0-9_]|%[0-9a-f]{2})+(?::[1-9][0-9]{0,3}|\*)?)*\})*$/i,
      // For the source: https://gist.github.com/dperini/729294
      // For test cases: https://mathiasbynens.be/demo/url-regex
      url: /^(?:https?|ftp):\/\/(?:\S+(?::\S*)?@)?(?:(?!(?:10|127)(?:\.\d{1,3}){3})(?!(?:169\.254|192\.168)(?:\.\d{1,3}){2})(?!172\.(?:1[6-9]|2\d|3[0-1])(?:\.\d{1,3}){2})(?:[1-9]\d?|1\d\d|2[01]\d|22[0-3])(?:\.(?:1?\d{1,2}|2[0-4]\d|25[0-5])){2}(?:\.(?:[1-9]\d?|1\d\d|2[0-4]\d|25[0-4]))|(?:(?:[a-z0-9\u{00a1}-\u{ffff}]+-)*[a-z0-9\u{00a1}-\u{ffff}]+)(?:\.(?:[a-z0-9\u{00a1}-\u{ffff}]+-)*[a-z0-9\u{00a1}-\u{ffff}]+)*(?:\.(?:[a-z\u{00a1}-\u{ffff}]{2,})))(?::\d{2,5})?(?:\/[^\s]*)?$/iu,
      email: /^[a-z0-9!#$%&'*+/=?^_`{|}~-]+(?:\.[a-z0-9!#$%&'*+/=?^_`{|}~-]+)*@(?:[a-z0-9](?:[a-z0-9-]*[a-z0-9])?\.)+[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$/i,
      hostname: /^(?=.{1,253}\.?$)[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[-0-9a-z]{0,61}[0-9a-z])?)*\.?$/i,
      // optimized https://www.safaribooksonline.com/library/view/regular-expressions-cookbook/9780596802837/ch07s16.html
      ipv4: /^(?:(?:25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)\.){3}(?:25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)$/,
      ipv6: /^((([0-9a-f]{1,4}:){7}([0-9a-f]{1,4}|:))|(([0-9a-f]{1,4}:){6}(:[0-9a-f]{1,4}|((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3})|:))|(([0-9a-f]{1,4}:){5}(((:[0-9a-f]{1,4}){1,2})|:((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3})|:))|(([0-9a-f]{1,4}:){4}(((:[0-9a-f]{1,4}){1,3})|((:[0-9a-f]{1,4})?:((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}))|:))|(([0-9a-f]{1,4}:){3}(((:[0-9a-f]{1,4}){1,4})|((:[0-9a-f]{1,4}){0,2}:((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}))|:))|(([0-9a-f]{1,4}:){2}(((:[0-9a-f]{1,4}){1,5})|((:[0-9a-f]{1,4}){0,3}:((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}))|:))|(([0-9a-f]{1,4}:){1}(((:[0-9a-f]{1,4}){1,6})|((:[0-9a-f]{1,4}){0,4}:((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}))|:))|(:(((:[0-9a-f]{1,4}){1,7})|((:[0-9a-f]{1,4}){0,5}:((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}))|:)))$/i,
      regex,
      // uuid: http://tools.ietf.org/html/rfc4122
      uuid: /^(?:urn:uuid:)?[0-9a-f]{8}-(?:[0-9a-f]{4}-){3}[0-9a-f]{12}$/i,
      // JSON-pointer: https://tools.ietf.org/html/rfc6901
      // uri fragment: https://tools.ietf.org/html/rfc3986#appendix-A
      "json-pointer": /^(?:\/(?:[^~/]|~0|~1)*)*$/,
      "json-pointer-uri-fragment": /^#(?:\/(?:[a-z0-9_\-.!$&'()*+,;:=@]|%[0-9a-f]{2}|~0|~1)*)*$/i,
      // relative JSON-pointer: http://tools.ietf.org/html/draft-luff-relative-json-pointer-00
      "relative-json-pointer": /^(?:0|[1-9][0-9]*)(?:#|(?:\/(?:[^~/]|~0|~1)*)*)$/,
      // the following formats are used by the openapi specification: https://spec.openapis.org/oas/v3.0.0#data-types
      // byte: https://github.com/miguelmota/is-base64
      byte,
      // signed 32 bit integer
      int32: { type: "number", validate: validateInt32 },
      // signed 64 bit integer
      int64: { type: "number", validate: validateInt64 },
      // C-type float
      float: { type: "number", validate: validateNumber },
      // C-type double
      double: { type: "number", validate: validateNumber },
      // hint to the UI to hide input strings
      password: true,
      // unchecked string payload
      binary: true
    };
    exports.fastFormats = {
      ...exports.fullFormats,
      date: fmtDef(/^\d\d\d\d-[0-1]\d-[0-3]\d$/, compareDate),
      time: fmtDef(/^(?:[0-2]\d:[0-5]\d:[0-5]\d|23:59:60)(?:\.\d+)?(?:z|[+-]\d\d(?::?\d\d)?)$/i, compareTime),
      "date-time": fmtDef(/^\d\d\d\d-[0-1]\d-[0-3]\dt(?:[0-2]\d:[0-5]\d:[0-5]\d|23:59:60)(?:\.\d+)?(?:z|[+-]\d\d(?::?\d\d)?)$/i, compareDateTime),
      "iso-time": fmtDef(/^(?:[0-2]\d:[0-5]\d:[0-5]\d|23:59:60)(?:\.\d+)?(?:z|[+-]\d\d(?::?\d\d)?)?$/i, compareIsoTime),
      "iso-date-time": fmtDef(/^\d\d\d\d-[0-1]\d-[0-3]\d[t\s](?:[0-2]\d:[0-5]\d:[0-5]\d|23:59:60)(?:\.\d+)?(?:z|[+-]\d\d(?::?\d\d)?)?$/i, compareIsoDateTime),
      // uri: https://github.com/mafintosh/is-my-json-valid/blob/master/formats.js
      uri: /^(?:[a-z][a-z0-9+\-.]*:)(?:\/?\/)?[^\s]*$/i,
      "uri-reference": /^(?:(?:[a-z][a-z0-9+\-.]*:)?\/?\/)?(?:[^\\\s#][^\s#]*)?(?:#[^\\\s]*)?$/i,
      // email (sources from jsen validator):
      // http://stackoverflow.com/questions/201323/using-a-regular-expression-to-validate-an-email-address#answer-8829363
      // http://www.w3.org/TR/html5/forms.html#valid-e-mail-address (search for 'wilful violation')
      email: /^[a-z0-9.!#$%&'*+/=?^_`{|}~-]+@[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)*$/i
    };
    exports.formatNames = Object.keys(exports.fullFormats);
    function isLeapYear(year) {
      return year % 4 === 0 && (year % 100 !== 0 || year % 400 === 0);
    }
    var DATE = /^(\d\d\d\d)-(\d\d)-(\d\d)$/;
    var DAYS = [0, 31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31];
    function date(str) {
      const matches = DATE.exec(str);
      if (!matches)
        return false;
      const year = +matches[1];
      const month = +matches[2];
      const day = +matches[3];
      return month >= 1 && month <= 12 && day >= 1 && day <= (month === 2 && isLeapYear(year) ? 29 : DAYS[month]);
    }
    function compareDate(d1, d2) {
      if (!(d1 && d2))
        return void 0;
      if (d1 > d2)
        return 1;
      if (d1 < d2)
        return -1;
      return 0;
    }
    var TIME = /^(\d\d):(\d\d):(\d\d(?:\.\d+)?)(z|([+-])(\d\d)(?::?(\d\d))?)?$/i;
    function getTime(strictTimeZone) {
      return function time(str) {
        const matches = TIME.exec(str);
        if (!matches)
          return false;
        const hr = +matches[1];
        const min = +matches[2];
        const sec = +matches[3];
        const tz = matches[4];
        const tzSign = matches[5] === "-" ? -1 : 1;
        const tzH = +(matches[6] || 0);
        const tzM = +(matches[7] || 0);
        if (tzH > 23 || tzM > 59 || strictTimeZone && !tz)
          return false;
        if (hr <= 23 && min <= 59 && sec < 60)
          return true;
        const utcMin = min - tzM * tzSign;
        const utcHr = hr - tzH * tzSign - (utcMin < 0 ? 1 : 0);
        return (utcHr === 23 || utcHr === -1) && (utcMin === 59 || utcMin === -1) && sec < 61;
      };
    }
    function compareTime(s1, s2) {
      if (!(s1 && s2))
        return void 0;
      const t1 = (/* @__PURE__ */ new Date("2020-01-01T" + s1)).valueOf();
      const t2 = (/* @__PURE__ */ new Date("2020-01-01T" + s2)).valueOf();
      if (!(t1 && t2))
        return void 0;
      return t1 - t2;
    }
    function compareIsoTime(t1, t2) {
      if (!(t1 && t2))
        return void 0;
      const a1 = TIME.exec(t1);
      const a2 = TIME.exec(t2);
      if (!(a1 && a2))
        return void 0;
      t1 = a1[1] + a1[2] + a1[3];
      t2 = a2[1] + a2[2] + a2[3];
      if (t1 > t2)
        return 1;
      if (t1 < t2)
        return -1;
      return 0;
    }
    var DATE_TIME_SEPARATOR = /t|\s/i;
    function getDateTime(strictTimeZone) {
      const time = getTime(strictTimeZone);
      return function date_time(str) {
        const dateTime = str.split(DATE_TIME_SEPARATOR);
        return dateTime.length === 2 && date(dateTime[0]) && time(dateTime[1]);
      };
    }
    function compareDateTime(dt1, dt2) {
      if (!(dt1 && dt2))
        return void 0;
      const d1 = new Date(dt1).valueOf();
      const d2 = new Date(dt2).valueOf();
      if (!(d1 && d2))
        return void 0;
      return d1 - d2;
    }
    function compareIsoDateTime(dt1, dt2) {
      if (!(dt1 && dt2))
        return void 0;
      const [d1, t1] = dt1.split(DATE_TIME_SEPARATOR);
      const [d2, t2] = dt2.split(DATE_TIME_SEPARATOR);
      const res = compareDate(d1, d2);
      if (res === void 0)
        return void 0;
      return res || compareTime(t1, t2);
    }
    var NOT_URI_FRAGMENT = /\/|:/;
    var URI = /^(?:[a-z][a-z0-9+\-.]*:)(?:\/?\/(?:(?:[a-z0-9\-._~!$&'()*+,;=:]|%[0-9a-f]{2})*@)?(?:\[(?:(?:(?:(?:[0-9a-f]{1,4}:){6}|::(?:[0-9a-f]{1,4}:){5}|(?:[0-9a-f]{1,4})?::(?:[0-9a-f]{1,4}:){4}|(?:(?:[0-9a-f]{1,4}:){0,1}[0-9a-f]{1,4})?::(?:[0-9a-f]{1,4}:){3}|(?:(?:[0-9a-f]{1,4}:){0,2}[0-9a-f]{1,4})?::(?:[0-9a-f]{1,4}:){2}|(?:(?:[0-9a-f]{1,4}:){0,3}[0-9a-f]{1,4})?::[0-9a-f]{1,4}:|(?:(?:[0-9a-f]{1,4}:){0,4}[0-9a-f]{1,4})?::)(?:[0-9a-f]{1,4}:[0-9a-f]{1,4}|(?:(?:25[0-5]|2[0-4]\d|[01]?\d\d?)\.){3}(?:25[0-5]|2[0-4]\d|[01]?\d\d?))|(?:(?:[0-9a-f]{1,4}:){0,5}[0-9a-f]{1,4})?::[0-9a-f]{1,4}|(?:(?:[0-9a-f]{1,4}:){0,6}[0-9a-f]{1,4})?::)|[Vv][0-9a-f]+\.[a-z0-9\-._~!$&'()*+,;=:]+)\]|(?:(?:25[0-5]|2[0-4]\d|[01]?\d\d?)\.){3}(?:25[0-5]|2[0-4]\d|[01]?\d\d?)|(?:[a-z0-9\-._~!$&'()*+,;=]|%[0-9a-f]{2})*)(?::\d*)?(?:\/(?:[a-z0-9\-._~!$&'()*+,;=:@]|%[0-9a-f]{2})*)*|\/(?:(?:[a-z0-9\-._~!$&'()*+,;=:@]|%[0-9a-f]{2})+(?:\/(?:[a-z0-9\-._~!$&'()*+,;=:@]|%[0-9a-f]{2})*)*)?|(?:[a-z0-9\-._~!$&'()*+,;=:@]|%[0-9a-f]{2})+(?:\/(?:[a-z0-9\-._~!$&'()*+,;=:@]|%[0-9a-f]{2})*)*)(?:\?(?:[a-z0-9\-._~!$&'()*+,;=:@/?]|%[0-9a-f]{2})*)?(?:#(?:[a-z0-9\-._~!$&'()*+,;=:@/?]|%[0-9a-f]{2})*)?$/i;
    function uri(str) {
      return NOT_URI_FRAGMENT.test(str) && URI.test(str);
    }
    var BYTE = /^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$/gm;
    function byte(str) {
      BYTE.lastIndex = 0;
      return BYTE.test(str);
    }
    var MIN_INT32 = -(2 ** 31);
    var MAX_INT32 = 2 ** 31 - 1;
    function validateInt32(value) {
      return Number.isInteger(value) && value <= MAX_INT32 && value >= MIN_INT32;
    }
    function validateInt64(value) {
      return Number.isInteger(value);
    }
    function validateNumber() {
      return true;
    }
    var Z_ANCHOR = /[^\\]\\Z/;
    function regex(str) {
      if (Z_ANCHOR.test(str))
        return false;
      try {
        new RegExp(str);
        return true;
      } catch (e) {
        return false;
      }
    }
  }
});

// ../../../../../../tmp/atlas-protocol-validators/entity.mjs
var entity_default = validate20;
var func1 = require_ucs2length().default;
var formats0 = require_formats().fullFormats["date-time"];
var pattern4 = new RegExp("^custom_", "u");
function validate20(data, { instancePath = "", parentData, parentDataProperty, rootData = data, dynamicAnchors = {} } = {}) {
  ;
  let vErrors = null;
  let errors = 0;
  const evaluated0 = validate20.evaluated;
  if (evaluated0.dynamicProps) {
    evaluated0.props = void 0;
  }
  if (evaluated0.dynamicItems) {
    evaluated0.items = void 0;
  }
  if (data && typeof data == "object" && !Array.isArray(data)) {
    if (data.components !== void 0) {
      let data0 = data.components;
      if (data0 && typeof data0 == "object" && !Array.isArray(data0)) {
        if (data0.supported_commands !== void 0) {
          let data1 = data0.supported_commands;
          if (data1 && typeof data1 == "object" && !Array.isArray(data1)) {
            if (data1.commands === void 0) {
              const err0 = { instancePath: instancePath + "/components/supported_commands", schemaPath: "#/$defs/supportedCommands/required", keyword: "required", params: { missingProperty: "commands" }, message: "must have required property 'commands'" };
              if (vErrors === null) {
                vErrors = [err0];
              } else {
                vErrors.push(err0);
              }
              errors++;
            }
            for (const key0 in data1) {
              if (!(key0 === "commands")) {
                const err1 = { instancePath: instancePath + "/components/supported_commands", schemaPath: "#/$defs/supportedCommands/additionalProperties", keyword: "additionalProperties", params: { additionalProperty: key0 }, message: "must NOT have additional properties" };
                if (vErrors === null) {
                  vErrors = [err1];
                } else {
                  vErrors.push(err1);
                }
                errors++;
              }
            }
            if (data1.commands !== void 0) {
              let data2 = data1.commands;
              if (Array.isArray(data2)) {
                const len0 = data2.length;
                for (let i0 = 0; i0 < len0; i0++) {
                  let data3 = data2[i0];
                  if (typeof data3 === "string") {
                    if (func1(data3) < 1) {
                      const err2 = { instancePath: instancePath + "/components/supported_commands/commands/" + i0, schemaPath: "#/$defs/supportedCommands/properties/commands/items/minLength", keyword: "minLength", params: { limit: 1 }, message: "must NOT have fewer than 1 characters" };
                      if (vErrors === null) {
                        vErrors = [err2];
                      } else {
                        vErrors.push(err2);
                      }
                      errors++;
                    }
                  } else {
                    const err3 = { instancePath: instancePath + "/components/supported_commands/commands/" + i0, schemaPath: "#/$defs/supportedCommands/properties/commands/items/type", keyword: "type", params: { type: "string" }, message: "must be string" };
                    if (vErrors === null) {
                      vErrors = [err3];
                    } else {
                      vErrors.push(err3);
                    }
                    errors++;
                  }
                }
              } else {
                const err4 = { instancePath: instancePath + "/components/supported_commands/commands", schemaPath: "#/$defs/supportedCommands/properties/commands/type", keyword: "type", params: { type: "array" }, message: "must be array" };
                if (vErrors === null) {
                  vErrors = [err4];
                } else {
                  vErrors.push(err4);
                }
                errors++;
              }
            }
          } else {
            const err5 = { instancePath: instancePath + "/components/supported_commands", schemaPath: "#/$defs/supportedCommands/type", keyword: "type", params: { type: "object" }, message: "must be object" };
            if (vErrors === null) {
              vErrors = [err5];
            } else {
              vErrors.push(err5);
            }
            errors++;
          }
        }
        if (data0.telemetry !== void 0) {
          let data4 = data0.telemetry;
          if (data4 && typeof data4 == "object" && !Array.isArray(data4)) {
            for (const key1 in data4) {
              if (!(key1 === "observed_at" || key1 === "latitude" || key1 === "longitude" || key1 === "altitude_m" || key1 === "speed_m_s" || key1 === "heading_deg" || key1 === "uncertainty_radius_m")) {
                const err6 = { instancePath: instancePath + "/components/telemetry", schemaPath: "#/$defs/telemetry/additionalProperties", keyword: "additionalProperties", params: { additionalProperty: key1 }, message: "must NOT have additional properties" };
                if (vErrors === null) {
                  vErrors = [err6];
                } else {
                  vErrors.push(err6);
                }
                errors++;
              }
            }
            if (data4.observed_at !== void 0) {
              let data5 = data4.observed_at;
              if (typeof data5 === "string") {
                if (!formats0.validate(data5)) {
                  const err7 = { instancePath: instancePath + "/components/telemetry/observed_at", schemaPath: "#/$defs/telemetry/properties/observed_at/format", keyword: "format", params: { format: "date-time" }, message: 'must match format "date-time"' };
                  if (vErrors === null) {
                    vErrors = [err7];
                  } else {
                    vErrors.push(err7);
                  }
                  errors++;
                }
              } else {
                const err8 = { instancePath: instancePath + "/components/telemetry/observed_at", schemaPath: "#/$defs/telemetry/properties/observed_at/type", keyword: "type", params: { type: "string" }, message: "must be string" };
                if (vErrors === null) {
                  vErrors = [err8];
                } else {
                  vErrors.push(err8);
                }
                errors++;
              }
            }
            if (data4.latitude !== void 0) {
              if (!(typeof data4.latitude == "number")) {
                const err9 = { instancePath: instancePath + "/components/telemetry/latitude", schemaPath: "#/$defs/telemetry/properties/latitude/type", keyword: "type", params: { type: "number" }, message: "must be number" };
                if (vErrors === null) {
                  vErrors = [err9];
                } else {
                  vErrors.push(err9);
                }
                errors++;
              }
            }
            if (data4.longitude !== void 0) {
              if (!(typeof data4.longitude == "number")) {
                const err10 = { instancePath: instancePath + "/components/telemetry/longitude", schemaPath: "#/$defs/telemetry/properties/longitude/type", keyword: "type", params: { type: "number" }, message: "must be number" };
                if (vErrors === null) {
                  vErrors = [err10];
                } else {
                  vErrors.push(err10);
                }
                errors++;
              }
            }
            if (data4.altitude_m !== void 0) {
              if (!(typeof data4.altitude_m == "number")) {
                const err11 = { instancePath: instancePath + "/components/telemetry/altitude_m", schemaPath: "#/$defs/telemetry/properties/altitude_m/type", keyword: "type", params: { type: "number" }, message: "must be number" };
                if (vErrors === null) {
                  vErrors = [err11];
                } else {
                  vErrors.push(err11);
                }
                errors++;
              }
            }
            if (data4.speed_m_s !== void 0) {
              if (!(typeof data4.speed_m_s == "number")) {
                const err12 = { instancePath: instancePath + "/components/telemetry/speed_m_s", schemaPath: "#/$defs/telemetry/properties/speed_m_s/type", keyword: "type", params: { type: "number" }, message: "must be number" };
                if (vErrors === null) {
                  vErrors = [err12];
                } else {
                  vErrors.push(err12);
                }
                errors++;
              }
            }
            if (data4.heading_deg !== void 0) {
              if (!(typeof data4.heading_deg == "number")) {
                const err13 = { instancePath: instancePath + "/components/telemetry/heading_deg", schemaPath: "#/$defs/telemetry/properties/heading_deg/type", keyword: "type", params: { type: "number" }, message: "must be number" };
                if (vErrors === null) {
                  vErrors = [err13];
                } else {
                  vErrors.push(err13);
                }
                errors++;
              }
            }
            if (data4.uncertainty_radius_m !== void 0) {
              if (!(typeof data4.uncertainty_radius_m == "number")) {
                const err14 = { instancePath: instancePath + "/components/telemetry/uncertainty_radius_m", schemaPath: "#/$defs/telemetry/properties/uncertainty_radius_m/type", keyword: "type", params: { type: "number" }, message: "must be number" };
                if (vErrors === null) {
                  vErrors = [err14];
                } else {
                  vErrors.push(err14);
                }
                errors++;
              }
            }
          } else {
            const err15 = { instancePath: instancePath + "/components/telemetry", schemaPath: "#/$defs/telemetry/type", keyword: "type", params: { type: "object" }, message: "must be object" };
            if (vErrors === null) {
              vErrors = [err15];
            } else {
              vErrors.push(err15);
            }
            errors++;
          }
        }
        if (data0.status !== void 0) {
          let data12 = data0.status;
          if (data12 && typeof data12 == "object" && !Array.isArray(data12)) {
            for (const key2 in data12) {
              if (!(key2 === "state" || key2 === "label" || key2 === "priority")) {
                const err16 = { instancePath: instancePath + "/components/status", schemaPath: "#/$defs/status/additionalProperties", keyword: "additionalProperties", params: { additionalProperty: key2 }, message: "must NOT have additional properties" };
                if (vErrors === null) {
                  vErrors = [err16];
                } else {
                  vErrors.push(err16);
                }
                errors++;
              }
            }
            if (data12.state !== void 0) {
              let data13 = data12.state;
              if (typeof data13 === "string") {
                if (func1(data13) < 1) {
                  const err17 = { instancePath: instancePath + "/components/status/state", schemaPath: "#/$defs/status/properties/state/minLength", keyword: "minLength", params: { limit: 1 }, message: "must NOT have fewer than 1 characters" };
                  if (vErrors === null) {
                    vErrors = [err17];
                  } else {
                    vErrors.push(err17);
                  }
                  errors++;
                }
              } else {
                const err18 = { instancePath: instancePath + "/components/status/state", schemaPath: "#/$defs/status/properties/state/type", keyword: "type", params: { type: "string" }, message: "must be string" };
                if (vErrors === null) {
                  vErrors = [err18];
                } else {
                  vErrors.push(err18);
                }
                errors++;
              }
            }
            if (data12.label !== void 0) {
              if (typeof data12.label !== "string") {
                const err19 = { instancePath: instancePath + "/components/status/label", schemaPath: "#/$defs/status/properties/label/type", keyword: "type", params: { type: "string" }, message: "must be string" };
                if (vErrors === null) {
                  vErrors = [err19];
                } else {
                  vErrors.push(err19);
                }
                errors++;
              }
            }
            if (data12.priority !== void 0) {
              let data15 = data12.priority;
              if (!(typeof data15 == "number" && (!(data15 % 1) && !isNaN(data15)))) {
                const err20 = { instancePath: instancePath + "/components/status/priority", schemaPath: "#/$defs/status/properties/priority/type", keyword: "type", params: { type: "integer" }, message: "must be integer" };
                if (vErrors === null) {
                  vErrors = [err20];
                } else {
                  vErrors.push(err20);
                }
                errors++;
              }
            }
          } else {
            const err21 = { instancePath: instancePath + "/components/status", schemaPath: "#/$defs/status/type", keyword: "type", params: { type: "object" }, message: "must be object" };
            if (vErrors === null) {
              vErrors = [err21];
            } else {
              vErrors.push(err21);
            }
            errors++;
          }
        }
        if (data0.geometry !== void 0) {
          let data16 = data0.geometry;
          if (data16 && typeof data16 == "object" && !Array.isArray(data16)) {
            if (data16.type === void 0) {
              const err22 = { instancePath: instancePath + "/components/geometry", schemaPath: "#/$defs/geometry/required", keyword: "required", params: { missingProperty: "type" }, message: "must have required property 'type'" };
              if (vErrors === null) {
                vErrors = [err22];
              } else {
                vErrors.push(err22);
              }
              errors++;
            }
            if (data16.coordinates === void 0) {
              const err23 = { instancePath: instancePath + "/components/geometry", schemaPath: "#/$defs/geometry/required", keyword: "required", params: { missingProperty: "coordinates" }, message: "must have required property 'coordinates'" };
              if (vErrors === null) {
                vErrors = [err23];
              } else {
                vErrors.push(err23);
              }
              errors++;
            }
            for (const key3 in data16) {
              if (!(key3 === "type" || key3 === "coordinates")) {
                const err24 = { instancePath: instancePath + "/components/geometry", schemaPath: "#/$defs/geometry/additionalProperties", keyword: "additionalProperties", params: { additionalProperty: key3 }, message: "must NOT have additional properties" };
                if (vErrors === null) {
                  vErrors = [err24];
                } else {
                  vErrors.push(err24);
                }
                errors++;
              }
            }
            if (data16.type !== void 0) {
              let data17 = data16.type;
              if (typeof data17 === "string") {
                if (func1(data17) < 1) {
                  const err25 = { instancePath: instancePath + "/components/geometry/type", schemaPath: "#/$defs/geometry/properties/type/minLength", keyword: "minLength", params: { limit: 1 }, message: "must NOT have fewer than 1 characters" };
                  if (vErrors === null) {
                    vErrors = [err25];
                  } else {
                    vErrors.push(err25);
                  }
                  errors++;
                }
              } else {
                const err26 = { instancePath: instancePath + "/components/geometry/type", schemaPath: "#/$defs/geometry/properties/type/type", keyword: "type", params: { type: "string" }, message: "must be string" };
                if (vErrors === null) {
                  vErrors = [err26];
                } else {
                  vErrors.push(err26);
                }
                errors++;
              }
            }
            if (data16.coordinates !== void 0) {
              if (!Array.isArray(data16.coordinates)) {
                const err27 = { instancePath: instancePath + "/components/geometry/coordinates", schemaPath: "#/$defs/geometry/properties/coordinates/type", keyword: "type", params: { type: "array" }, message: "must be array" };
                if (vErrors === null) {
                  vErrors = [err27];
                } else {
                  vErrors.push(err27);
                }
                errors++;
              }
            }
          } else {
            const err28 = { instancePath: instancePath + "/components/geometry", schemaPath: "#/$defs/geometry/type", keyword: "type", params: { type: "object" }, message: "must be object" };
            if (vErrors === null) {
              vErrors = [err28];
            } else {
              vErrors.push(err28);
            }
            errors++;
          }
        }
        if (data0.heartbeat !== void 0) {
          let data19 = data0.heartbeat;
          if (data19 && typeof data19 == "object" && !Array.isArray(data19)) {
            for (const key4 in data19) {
              if (!(key4 === "observed_at" || key4 === "source" || key4 === "sequence")) {
                const err29 = { instancePath: instancePath + "/components/heartbeat", schemaPath: "#/$defs/heartbeat/additionalProperties", keyword: "additionalProperties", params: { additionalProperty: key4 }, message: "must NOT have additional properties" };
                if (vErrors === null) {
                  vErrors = [err29];
                } else {
                  vErrors.push(err29);
                }
                errors++;
              }
            }
            if (data19.observed_at !== void 0) {
              let data20 = data19.observed_at;
              if (typeof data20 === "string") {
                if (!formats0.validate(data20)) {
                  const err30 = { instancePath: instancePath + "/components/heartbeat/observed_at", schemaPath: "#/$defs/heartbeat/properties/observed_at/format", keyword: "format", params: { format: "date-time" }, message: 'must match format "date-time"' };
                  if (vErrors === null) {
                    vErrors = [err30];
                  } else {
                    vErrors.push(err30);
                  }
                  errors++;
                }
              } else {
                const err31 = { instancePath: instancePath + "/components/heartbeat/observed_at", schemaPath: "#/$defs/heartbeat/properties/observed_at/type", keyword: "type", params: { type: "string" }, message: "must be string" };
                if (vErrors === null) {
                  vErrors = [err31];
                } else {
                  vErrors.push(err31);
                }
                errors++;
              }
            }
            if (data19.source !== void 0) {
              if (typeof data19.source !== "string") {
                const err32 = { instancePath: instancePath + "/components/heartbeat/source", schemaPath: "#/$defs/heartbeat/properties/source/type", keyword: "type", params: { type: "string" }, message: "must be string" };
                if (vErrors === null) {
                  vErrors = [err32];
                } else {
                  vErrors.push(err32);
                }
                errors++;
              }
            }
            if (data19.sequence !== void 0) {
              let data22 = data19.sequence;
              if (!(typeof data22 == "number" && (!(data22 % 1) && !isNaN(data22)))) {
                const err33 = { instancePath: instancePath + "/components/heartbeat/sequence", schemaPath: "#/$defs/heartbeat/properties/sequence/type", keyword: "type", params: { type: "integer" }, message: "must be integer" };
                if (vErrors === null) {
                  vErrors = [err33];
                } else {
                  vErrors.push(err33);
                }
                errors++;
              }
            }
          } else {
            const err34 = { instancePath: instancePath + "/components/heartbeat", schemaPath: "#/$defs/heartbeat/type", keyword: "type", params: { type: "object" }, message: "must be object" };
            if (vErrors === null) {
              vErrors = [err34];
            } else {
              vErrors.push(err34);
            }
            errors++;
          }
        }
        if (data0.health !== void 0) {
          let data23 = data0.health;
          if (data23 && typeof data23 == "object" && !Array.isArray(data23)) {
            for (const key5 in data23) {
              if (!(key5 === "state" || key5 === "battery_percent" || key5 === "faults")) {
                const err35 = { instancePath: instancePath + "/components/health", schemaPath: "#/$defs/health/additionalProperties", keyword: "additionalProperties", params: { additionalProperty: key5 }, message: "must NOT have additional properties" };
                if (vErrors === null) {
                  vErrors = [err35];
                } else {
                  vErrors.push(err35);
                }
                errors++;
              }
            }
            if (data23.state !== void 0) {
              let data24 = data23.state;
              if (typeof data24 === "string") {
                if (func1(data24) < 1) {
                  const err36 = { instancePath: instancePath + "/components/health/state", schemaPath: "#/$defs/health/properties/state/minLength", keyword: "minLength", params: { limit: 1 }, message: "must NOT have fewer than 1 characters" };
                  if (vErrors === null) {
                    vErrors = [err36];
                  } else {
                    vErrors.push(err36);
                  }
                  errors++;
                }
              } else {
                const err37 = { instancePath: instancePath + "/components/health/state", schemaPath: "#/$defs/health/properties/state/type", keyword: "type", params: { type: "string" }, message: "must be string" };
                if (vErrors === null) {
                  vErrors = [err37];
                } else {
                  vErrors.push(err37);
                }
                errors++;
              }
            }
            if (data23.battery_percent !== void 0) {
              if (!(typeof data23.battery_percent == "number")) {
                const err38 = { instancePath: instancePath + "/components/health/battery_percent", schemaPath: "#/$defs/health/properties/battery_percent/type", keyword: "type", params: { type: "number" }, message: "must be number" };
                if (vErrors === null) {
                  vErrors = [err38];
                } else {
                  vErrors.push(err38);
                }
                errors++;
              }
            }
            if (data23.faults !== void 0) {
              let data26 = data23.faults;
              if (Array.isArray(data26)) {
                const len1 = data26.length;
                for (let i1 = 0; i1 < len1; i1++) {
                  if (typeof data26[i1] !== "string") {
                    const err39 = { instancePath: instancePath + "/components/health/faults/" + i1, schemaPath: "#/$defs/health/properties/faults/items/type", keyword: "type", params: { type: "string" }, message: "must be string" };
                    if (vErrors === null) {
                      vErrors = [err39];
                    } else {
                      vErrors.push(err39);
                    }
                    errors++;
                  }
                }
              } else {
                const err40 = { instancePath: instancePath + "/components/health/faults", schemaPath: "#/$defs/health/properties/faults/type", keyword: "type", params: { type: "array" }, message: "must be array" };
                if (vErrors === null) {
                  vErrors = [err40];
                } else {
                  vErrors.push(err40);
                }
                errors++;
              }
            }
          } else {
            const err41 = { instancePath: instancePath + "/components/health", schemaPath: "#/$defs/health/type", keyword: "type", params: { type: "object" }, message: "must be object" };
            if (vErrors === null) {
              vErrors = [err41];
            } else {
              vErrors.push(err41);
            }
            errors++;
          }
        }
        if (data0.communications !== void 0) {
          let data28 = data0.communications;
          if (data28 && typeof data28 == "object" && !Array.isArray(data28)) {
            for (const key6 in data28) {
              if (!(key6 === "links")) {
                const err42 = { instancePath: instancePath + "/components/communications", schemaPath: "#/$defs/communications/additionalProperties", keyword: "additionalProperties", params: { additionalProperty: key6 }, message: "must NOT have additional properties" };
                if (vErrors === null) {
                  vErrors = [err42];
                } else {
                  vErrors.push(err42);
                }
                errors++;
              }
            }
            if (data28.links !== void 0) {
              let data29 = data28.links;
              if (Array.isArray(data29)) {
                const len2 = data29.length;
                for (let i2 = 0; i2 < len2; i2++) {
                  let data30 = data29[i2];
                  if (data30 && typeof data30 == "object" && !Array.isArray(data30)) {
                    for (const key7 in data30) {
                      if (!(key7 === "type" || key7 === "status" || key7 === "address" || key7 === "rssi_dbm" || key7 === "snr_db")) {
                        const err43 = { instancePath: instancePath + "/components/communications/links/" + i2, schemaPath: "#/$defs/communications/properties/links/items/additionalProperties", keyword: "additionalProperties", params: { additionalProperty: key7 }, message: "must NOT have additional properties" };
                        if (vErrors === null) {
                          vErrors = [err43];
                        } else {
                          vErrors.push(err43);
                        }
                        errors++;
                      }
                    }
                    if (data30.type !== void 0) {
                      let data31 = data30.type;
                      if (typeof data31 === "string") {
                        if (func1(data31) < 1) {
                          const err44 = { instancePath: instancePath + "/components/communications/links/" + i2 + "/type", schemaPath: "#/$defs/communications/properties/links/items/properties/type/minLength", keyword: "minLength", params: { limit: 1 }, message: "must NOT have fewer than 1 characters" };
                          if (vErrors === null) {
                            vErrors = [err44];
                          } else {
                            vErrors.push(err44);
                          }
                          errors++;
                        }
                      } else {
                        const err45 = { instancePath: instancePath + "/components/communications/links/" + i2 + "/type", schemaPath: "#/$defs/communications/properties/links/items/properties/type/type", keyword: "type", params: { type: "string" }, message: "must be string" };
                        if (vErrors === null) {
                          vErrors = [err45];
                        } else {
                          vErrors.push(err45);
                        }
                        errors++;
                      }
                    }
                    if (data30.status !== void 0) {
                      let data32 = data30.status;
                      if (typeof data32 === "string") {
                        if (func1(data32) < 1) {
                          const err46 = { instancePath: instancePath + "/components/communications/links/" + i2 + "/status", schemaPath: "#/$defs/communications/properties/links/items/properties/status/minLength", keyword: "minLength", params: { limit: 1 }, message: "must NOT have fewer than 1 characters" };
                          if (vErrors === null) {
                            vErrors = [err46];
                          } else {
                            vErrors.push(err46);
                          }
                          errors++;
                        }
                      } else {
                        const err47 = { instancePath: instancePath + "/components/communications/links/" + i2 + "/status", schemaPath: "#/$defs/communications/properties/links/items/properties/status/type", keyword: "type", params: { type: "string" }, message: "must be string" };
                        if (vErrors === null) {
                          vErrors = [err47];
                        } else {
                          vErrors.push(err47);
                        }
                        errors++;
                      }
                    }
                    if (data30.address !== void 0) {
                      if (typeof data30.address !== "string") {
                        const err48 = { instancePath: instancePath + "/components/communications/links/" + i2 + "/address", schemaPath: "#/$defs/communications/properties/links/items/properties/address/type", keyword: "type", params: { type: "string" }, message: "must be string" };
                        if (vErrors === null) {
                          vErrors = [err48];
                        } else {
                          vErrors.push(err48);
                        }
                        errors++;
                      }
                    }
                    if (data30.rssi_dbm !== void 0) {
                      if (!(typeof data30.rssi_dbm == "number")) {
                        const err49 = { instancePath: instancePath + "/components/communications/links/" + i2 + "/rssi_dbm", schemaPath: "#/$defs/communications/properties/links/items/properties/rssi_dbm/type", keyword: "type", params: { type: "number" }, message: "must be number" };
                        if (vErrors === null) {
                          vErrors = [err49];
                        } else {
                          vErrors.push(err49);
                        }
                        errors++;
                      }
                    }
                    if (data30.snr_db !== void 0) {
                      if (!(typeof data30.snr_db == "number")) {
                        const err50 = { instancePath: instancePath + "/components/communications/links/" + i2 + "/snr_db", schemaPath: "#/$defs/communications/properties/links/items/properties/snr_db/type", keyword: "type", params: { type: "number" }, message: "must be number" };
                        if (vErrors === null) {
                          vErrors = [err50];
                        } else {
                          vErrors.push(err50);
                        }
                        errors++;
                      }
                    }
                  } else {
                    const err51 = { instancePath: instancePath + "/components/communications/links/" + i2, schemaPath: "#/$defs/communications/properties/links/items/type", keyword: "type", params: { type: "object" }, message: "must be object" };
                    if (vErrors === null) {
                      vErrors = [err51];
                    } else {
                      vErrors.push(err51);
                    }
                    errors++;
                  }
                }
              } else {
                const err52 = { instancePath: instancePath + "/components/communications/links", schemaPath: "#/$defs/communications/properties/links/type", keyword: "type", params: { type: "array" }, message: "must be array" };
                if (vErrors === null) {
                  vErrors = [err52];
                } else {
                  vErrors.push(err52);
                }
                errors++;
              }
            }
          } else {
            const err53 = { instancePath: instancePath + "/components/communications", schemaPath: "#/$defs/communications/type", keyword: "type", params: { type: "object" }, message: "must be object" };
            if (vErrors === null) {
              vErrors = [err53];
            } else {
              vErrors.push(err53);
            }
            errors++;
          }
        }
        if (data0.sensor_refs !== void 0) {
          let data36 = data0.sensor_refs;
          if (data36 && typeof data36 == "object" && !Array.isArray(data36)) {
            for (const key8 in data36) {
              if (!(key8 === "sensors")) {
                const err54 = { instancePath: instancePath + "/components/sensor_refs", schemaPath: "#/$defs/sensorRefs/additionalProperties", keyword: "additionalProperties", params: { additionalProperty: key8 }, message: "must NOT have additional properties" };
                if (vErrors === null) {
                  vErrors = [err54];
                } else {
                  vErrors.push(err54);
                }
                errors++;
              }
            }
            if (data36.sensors !== void 0) {
              let data37 = data36.sensors;
              if (Array.isArray(data37)) {
                const len3 = data37.length;
                for (let i3 = 0; i3 < len3; i3++) {
                  let data38 = data37[i3];
                  if (data38 && typeof data38 == "object" && !Array.isArray(data38)) {
                    for (const key9 in data38) {
                      if (!(key9 === "sensor_id" || key9 === "type" || key9 === "label" || key9 === "object_id" || key9 === "mount")) {
                        const err55 = { instancePath: instancePath + "/components/sensor_refs/sensors/" + i3, schemaPath: "#/$defs/sensorRefs/properties/sensors/items/additionalProperties", keyword: "additionalProperties", params: { additionalProperty: key9 }, message: "must NOT have additional properties" };
                        if (vErrors === null) {
                          vErrors = [err55];
                        } else {
                          vErrors.push(err55);
                        }
                        errors++;
                      }
                    }
                    if (data38.sensor_id !== void 0) {
                      let data39 = data38.sensor_id;
                      if (typeof data39 === "string") {
                        if (func1(data39) < 1) {
                          const err56 = { instancePath: instancePath + "/components/sensor_refs/sensors/" + i3 + "/sensor_id", schemaPath: "#/$defs/sensorRefs/properties/sensors/items/properties/sensor_id/minLength", keyword: "minLength", params: { limit: 1 }, message: "must NOT have fewer than 1 characters" };
                          if (vErrors === null) {
                            vErrors = [err56];
                          } else {
                            vErrors.push(err56);
                          }
                          errors++;
                        }
                      } else {
                        const err57 = { instancePath: instancePath + "/components/sensor_refs/sensors/" + i3 + "/sensor_id", schemaPath: "#/$defs/sensorRefs/properties/sensors/items/properties/sensor_id/type", keyword: "type", params: { type: "string" }, message: "must be string" };
                        if (vErrors === null) {
                          vErrors = [err57];
                        } else {
                          vErrors.push(err57);
                        }
                        errors++;
                      }
                    }
                    if (data38.type !== void 0) {
                      let data40 = data38.type;
                      if (typeof data40 === "string") {
                        if (func1(data40) < 1) {
                          const err58 = { instancePath: instancePath + "/components/sensor_refs/sensors/" + i3 + "/type", schemaPath: "#/$defs/sensorRefs/properties/sensors/items/properties/type/minLength", keyword: "minLength", params: { limit: 1 }, message: "must NOT have fewer than 1 characters" };
                          if (vErrors === null) {
                            vErrors = [err58];
                          } else {
                            vErrors.push(err58);
                          }
                          errors++;
                        }
                      } else {
                        const err59 = { instancePath: instancePath + "/components/sensor_refs/sensors/" + i3 + "/type", schemaPath: "#/$defs/sensorRefs/properties/sensors/items/properties/type/type", keyword: "type", params: { type: "string" }, message: "must be string" };
                        if (vErrors === null) {
                          vErrors = [err59];
                        } else {
                          vErrors.push(err59);
                        }
                        errors++;
                      }
                    }
                    if (data38.label !== void 0) {
                      if (typeof data38.label !== "string") {
                        const err60 = { instancePath: instancePath + "/components/sensor_refs/sensors/" + i3 + "/label", schemaPath: "#/$defs/sensorRefs/properties/sensors/items/properties/label/type", keyword: "type", params: { type: "string" }, message: "must be string" };
                        if (vErrors === null) {
                          vErrors = [err60];
                        } else {
                          vErrors.push(err60);
                        }
                        errors++;
                      }
                    }
                    if (data38.object_id !== void 0) {
                      if (typeof data38.object_id !== "string") {
                        const err61 = { instancePath: instancePath + "/components/sensor_refs/sensors/" + i3 + "/object_id", schemaPath: "#/$defs/sensorRefs/properties/sensors/items/properties/object_id/type", keyword: "type", params: { type: "string" }, message: "must be string" };
                        if (vErrors === null) {
                          vErrors = [err61];
                        } else {
                          vErrors.push(err61);
                        }
                        errors++;
                      }
                    }
                    if (data38.mount !== void 0) {
                      let data43 = data38.mount;
                      if (data43 && typeof data43 == "object" && !Array.isArray(data43)) {
                        for (const key10 in data43) {
                          if (!(key10 === "location" || key10 === "bearing_deg" || key10 === "elevation_deg" || key10 === "roll_deg")) {
                            const err62 = { instancePath: instancePath + "/components/sensor_refs/sensors/" + i3 + "/mount", schemaPath: "#/$defs/sensorRefs/properties/sensors/items/properties/mount/additionalProperties", keyword: "additionalProperties", params: { additionalProperty: key10 }, message: "must NOT have additional properties" };
                            if (vErrors === null) {
                              vErrors = [err62];
                            } else {
                              vErrors.push(err62);
                            }
                            errors++;
                          }
                        }
                        if (data43.location !== void 0) {
                          let data44 = data43.location;
                          if (typeof data44 === "string") {
                            if (func1(data44) < 1) {
                              const err63 = { instancePath: instancePath + "/components/sensor_refs/sensors/" + i3 + "/mount/location", schemaPath: "#/$defs/sensorRefs/properties/sensors/items/properties/mount/properties/location/minLength", keyword: "minLength", params: { limit: 1 }, message: "must NOT have fewer than 1 characters" };
                              if (vErrors === null) {
                                vErrors = [err63];
                              } else {
                                vErrors.push(err63);
                              }
                              errors++;
                            }
                          } else {
                            const err64 = { instancePath: instancePath + "/components/sensor_refs/sensors/" + i3 + "/mount/location", schemaPath: "#/$defs/sensorRefs/properties/sensors/items/properties/mount/properties/location/type", keyword: "type", params: { type: "string" }, message: "must be string" };
                            if (vErrors === null) {
                              vErrors = [err64];
                            } else {
                              vErrors.push(err64);
                            }
                            errors++;
                          }
                        }
                        if (data43.bearing_deg !== void 0) {
                          if (!(typeof data43.bearing_deg == "number")) {
                            const err65 = { instancePath: instancePath + "/components/sensor_refs/sensors/" + i3 + "/mount/bearing_deg", schemaPath: "#/$defs/sensorRefs/properties/sensors/items/properties/mount/properties/bearing_deg/type", keyword: "type", params: { type: "number" }, message: "must be number" };
                            if (vErrors === null) {
                              vErrors = [err65];
                            } else {
                              vErrors.push(err65);
                            }
                            errors++;
                          }
                        }
                        if (data43.elevation_deg !== void 0) {
                          if (!(typeof data43.elevation_deg == "number")) {
                            const err66 = { instancePath: instancePath + "/components/sensor_refs/sensors/" + i3 + "/mount/elevation_deg", schemaPath: "#/$defs/sensorRefs/properties/sensors/items/properties/mount/properties/elevation_deg/type", keyword: "type", params: { type: "number" }, message: "must be number" };
                            if (vErrors === null) {
                              vErrors = [err66];
                            } else {
                              vErrors.push(err66);
                            }
                            errors++;
                          }
                        }
                        if (data43.roll_deg !== void 0) {
                          if (!(typeof data43.roll_deg == "number")) {
                            const err67 = { instancePath: instancePath + "/components/sensor_refs/sensors/" + i3 + "/mount/roll_deg", schemaPath: "#/$defs/sensorRefs/properties/sensors/items/properties/mount/properties/roll_deg/type", keyword: "type", params: { type: "number" }, message: "must be number" };
                            if (vErrors === null) {
                              vErrors = [err67];
                            } else {
                              vErrors.push(err67);
                            }
                            errors++;
                          }
                        }
                      } else {
                        const err68 = { instancePath: instancePath + "/components/sensor_refs/sensors/" + i3 + "/mount", schemaPath: "#/$defs/sensorRefs/properties/sensors/items/properties/mount/type", keyword: "type", params: { type: "object" }, message: "must be object" };
                        if (vErrors === null) {
                          vErrors = [err68];
                        } else {
                          vErrors.push(err68);
                        }
                        errors++;
                      }
                    }
                  } else {
                    const err69 = { instancePath: instancePath + "/components/sensor_refs/sensors/" + i3, schemaPath: "#/$defs/sensorRefs/properties/sensors/items/type", keyword: "type", params: { type: "object" }, message: "must be object" };
                    if (vErrors === null) {
                      vErrors = [err69];
                    } else {
                      vErrors.push(err69);
                    }
                    errors++;
                  }
                }
              } else {
                const err70 = { instancePath: instancePath + "/components/sensor_refs/sensors", schemaPath: "#/$defs/sensorRefs/properties/sensors/type", keyword: "type", params: { type: "array" }, message: "must be array" };
                if (vErrors === null) {
                  vErrors = [err70];
                } else {
                  vErrors.push(err70);
                }
                errors++;
              }
            }
          } else {
            const err71 = { instancePath: instancePath + "/components/sensor_refs", schemaPath: "#/$defs/sensorRefs/type", keyword: "type", params: { type: "object" }, message: "must be object" };
            if (vErrors === null) {
              vErrors = [err71];
            } else {
              vErrors.push(err71);
            }
            errors++;
          }
        }
        if (data0.fusion_summary !== void 0) {
          let data48 = data0.fusion_summary;
          if (data48 && typeof data48 == "object" && !Array.isArray(data48)) {
            for (const key11 in data48) {
              if (!(key11 === "observed_at" || key11 === "source_count" || key11 === "confidence" || key11 === "provenance_object_id")) {
                const err72 = { instancePath: instancePath + "/components/fusion_summary", schemaPath: "#/$defs/fusionSummary/additionalProperties", keyword: "additionalProperties", params: { additionalProperty: key11 }, message: "must NOT have additional properties" };
                if (vErrors === null) {
                  vErrors = [err72];
                } else {
                  vErrors.push(err72);
                }
                errors++;
              }
            }
            if (data48.observed_at !== void 0) {
              let data49 = data48.observed_at;
              if (typeof data49 === "string") {
                if (!formats0.validate(data49)) {
                  const err73 = { instancePath: instancePath + "/components/fusion_summary/observed_at", schemaPath: "#/$defs/fusionSummary/properties/observed_at/format", keyword: "format", params: { format: "date-time" }, message: 'must match format "date-time"' };
                  if (vErrors === null) {
                    vErrors = [err73];
                  } else {
                    vErrors.push(err73);
                  }
                  errors++;
                }
              } else {
                const err74 = { instancePath: instancePath + "/components/fusion_summary/observed_at", schemaPath: "#/$defs/fusionSummary/properties/observed_at/type", keyword: "type", params: { type: "string" }, message: "must be string" };
                if (vErrors === null) {
                  vErrors = [err74];
                } else {
                  vErrors.push(err74);
                }
                errors++;
              }
            }
            if (data48.source_count !== void 0) {
              let data50 = data48.source_count;
              if (!(typeof data50 == "number" && (!(data50 % 1) && !isNaN(data50)))) {
                const err75 = { instancePath: instancePath + "/components/fusion_summary/source_count", schemaPath: "#/$defs/fusionSummary/properties/source_count/type", keyword: "type", params: { type: "integer" }, message: "must be integer" };
                if (vErrors === null) {
                  vErrors = [err75];
                } else {
                  vErrors.push(err75);
                }
                errors++;
              }
            }
            if (data48.confidence !== void 0) {
              if (!(typeof data48.confidence == "number")) {
                const err76 = { instancePath: instancePath + "/components/fusion_summary/confidence", schemaPath: "#/$defs/fusionSummary/properties/confidence/type", keyword: "type", params: { type: "number" }, message: "must be number" };
                if (vErrors === null) {
                  vErrors = [err76];
                } else {
                  vErrors.push(err76);
                }
                errors++;
              }
            }
            if (data48.provenance_object_id !== void 0) {
              if (typeof data48.provenance_object_id !== "string") {
                const err77 = { instancePath: instancePath + "/components/fusion_summary/provenance_object_id", schemaPath: "#/$defs/fusionSummary/properties/provenance_object_id/type", keyword: "type", params: { type: "string" }, message: "must be string" };
                if (vErrors === null) {
                  vErrors = [err77];
                } else {
                  vErrors.push(err77);
                }
                errors++;
              }
            }
          } else {
            const err78 = { instancePath: instancePath + "/components/fusion_summary", schemaPath: "#/$defs/fusionSummary/type", keyword: "type", params: { type: "object" }, message: "must be object" };
            if (vErrors === null) {
              vErrors = [err78];
            } else {
              vErrors.push(err78);
            }
            errors++;
          }
        }
        for (const key12 in data0) {
          if (pattern4.test(key12)) {
            let data53 = data0[key12];
            if (!(data53 && typeof data53 == "object" && !Array.isArray(data53))) {
              const err79 = { instancePath: instancePath + "/components/" + key12.replace(/~/g, "~0").replace(/\//g, "~1"), schemaPath: "#/$defs/customSection/type", keyword: "type", params: { type: "object" }, message: "must be object" };
              if (vErrors === null) {
                vErrors = [err79];
              } else {
                vErrors.push(err79);
              }
              errors++;
            }
          }
        }
      } else {
        const err80 = { instancePath: instancePath + "/components", schemaPath: "#/properties/components/type", keyword: "type", params: { type: "object" }, message: "must be object" };
        if (vErrors === null) {
          vErrors = [err80];
        } else {
          vErrors.push(err80);
        }
        errors++;
      }
    }
    if (data.extra !== void 0) {
      let data54 = data.extra;
      if (!(data54 && typeof data54 == "object" && !Array.isArray(data54))) {
        const err81 = { instancePath: instancePath + "/extra", schemaPath: "#/properties/extra/type", keyword: "type", params: { type: "object" }, message: "must be object" };
        if (vErrors === null) {
          vErrors = [err81];
        } else {
          vErrors.push(err81);
        }
        errors++;
      }
    }
    for (const key13 in data) {
      if (pattern4.test(key13)) {
        let data55 = data[key13];
        if (!(data55 && typeof data55 == "object" && !Array.isArray(data55))) {
          const err82 = { instancePath: instancePath + "/" + key13.replace(/~/g, "~0").replace(/\//g, "~1"), schemaPath: "#/$defs/customSection/type", keyword: "type", params: { type: "object" }, message: "must be object" };
          if (vErrors === null) {
            vErrors = [err82];
          } else {
            vErrors.push(err82);
          }
          errors++;
        }
      }
    }
  } else {
    const err83 = { instancePath, schemaPath: "#/type", keyword: "type", params: { type: "object" }, message: "must be object" };
    if (vErrors === null) {
      vErrors = [err83];
    } else {
      vErrors.push(err83);
    }
    errors++;
  }
  validate20.errors = vErrors;
  return errors === 0;
}
validate20.evaluated = { "props": true, "dynamicProps": false, "dynamicItems": false };

// ../../../../../../tmp/atlas-protocol-validators/object.mjs
var object_default = validate202;
var formats02 = require_formats().fullFormats["date-time"];
var func12 = require_ucs2length().default;
var schema32 = { "type": "object", "required": ["type"], "additionalProperties": false, "properties": { "type": { "type": "string", "enum": ["object", "array", "string", "number", "integer", "boolean"] }, "properties": { "type": "object", "additionalProperties": { "$ref": "#/$defs/parameterSchema" } }, "required": { "type": "array", "items": { "type": "string", "minLength": 1 }, "uniqueItems": true }, "additionalProperties": { "type": "boolean" }, "items": { "$ref": "#/$defs/parameterSchema" }, "enum": { "type": "array" } } };
var wrapper0 = { validate: validate21 };
function validate21(data, { instancePath = "", parentData, parentDataProperty, rootData = data, dynamicAnchors = {} } = {}) {
  let vErrors = null;
  let errors = 0;
  const evaluated0 = validate21.evaluated;
  if (evaluated0.dynamicProps) {
    evaluated0.props = void 0;
  }
  if (evaluated0.dynamicItems) {
    evaluated0.items = void 0;
  }
  if (data && typeof data == "object" && !Array.isArray(data)) {
    if (data.type === void 0) {
      const err0 = { instancePath, schemaPath: "#/required", keyword: "required", params: { missingProperty: "type" }, message: "must have required property 'type'" };
      if (vErrors === null) {
        vErrors = [err0];
      } else {
        vErrors.push(err0);
      }
      errors++;
    }
    for (const key0 in data) {
      if (!(key0 === "type" || key0 === "properties" || key0 === "required" || key0 === "additionalProperties" || key0 === "items" || key0 === "enum")) {
        const err1 = { instancePath, schemaPath: "#/additionalProperties", keyword: "additionalProperties", params: { additionalProperty: key0 }, message: "must NOT have additional properties" };
        if (vErrors === null) {
          vErrors = [err1];
        } else {
          vErrors.push(err1);
        }
        errors++;
      }
    }
    if (data.type !== void 0) {
      let data0 = data.type;
      if (typeof data0 !== "string") {
        const err2 = { instancePath: instancePath + "/type", schemaPath: "#/properties/type/type", keyword: "type", params: { type: "string" }, message: "must be string" };
        if (vErrors === null) {
          vErrors = [err2];
        } else {
          vErrors.push(err2);
        }
        errors++;
      }
      if (!(data0 === "object" || data0 === "array" || data0 === "string" || data0 === "number" || data0 === "integer" || data0 === "boolean")) {
        const err3 = { instancePath: instancePath + "/type", schemaPath: "#/properties/type/enum", keyword: "enum", params: { allowedValues: schema32.properties.type.enum }, message: "must be equal to one of the allowed values" };
        if (vErrors === null) {
          vErrors = [err3];
        } else {
          vErrors.push(err3);
        }
        errors++;
      }
    }
    if (data.properties !== void 0) {
      let data1 = data.properties;
      if (data1 && typeof data1 == "object" && !Array.isArray(data1)) {
        for (const key1 in data1) {
          if (!wrapper0.validate(data1[key1], { instancePath: instancePath + "/properties/" + key1.replace(/~/g, "~0").replace(/\//g, "~1"), parentData: data1, parentDataProperty: key1, rootData, dynamicAnchors })) {
            vErrors = vErrors === null ? wrapper0.validate.errors : vErrors.concat(wrapper0.validate.errors);
            errors = vErrors.length;
          }
        }
      } else {
        const err4 = { instancePath: instancePath + "/properties", schemaPath: "#/properties/properties/type", keyword: "type", params: { type: "object" }, message: "must be object" };
        if (vErrors === null) {
          vErrors = [err4];
        } else {
          vErrors.push(err4);
        }
        errors++;
      }
    }
    if (data.required !== void 0) {
      let data3 = data.required;
      if (Array.isArray(data3)) {
        const len0 = data3.length;
        for (let i0 = 0; i0 < len0; i0++) {
          let data4 = data3[i0];
          if (typeof data4 === "string") {
            if (func12(data4) < 1) {
              const err5 = { instancePath: instancePath + "/required/" + i0, schemaPath: "#/properties/required/items/minLength", keyword: "minLength", params: { limit: 1 }, message: "must NOT have fewer than 1 characters" };
              if (vErrors === null) {
                vErrors = [err5];
              } else {
                vErrors.push(err5);
              }
              errors++;
            }
          } else {
            const err6 = { instancePath: instancePath + "/required/" + i0, schemaPath: "#/properties/required/items/type", keyword: "type", params: { type: "string" }, message: "must be string" };
            if (vErrors === null) {
              vErrors = [err6];
            } else {
              vErrors.push(err6);
            }
            errors++;
          }
        }
        let i1 = data3.length;
        let j0;
        if (i1 > 1) {
          const indices0 = {};
          for (; i1--; ) {
            let item0 = data3[i1];
            if (typeof item0 !== "string") {
              continue;
            }
            if (typeof indices0[item0] == "number") {
              j0 = indices0[item0];
              const err7 = { instancePath: instancePath + "/required", schemaPath: "#/properties/required/uniqueItems", keyword: "uniqueItems", params: { i: i1, j: j0 }, message: "must NOT have duplicate items (items ## " + j0 + " and " + i1 + " are identical)" };
              if (vErrors === null) {
                vErrors = [err7];
              } else {
                vErrors.push(err7);
              }
              errors++;
              break;
            }
            indices0[item0] = i1;
          }
        }
      } else {
        const err8 = { instancePath: instancePath + "/required", schemaPath: "#/properties/required/type", keyword: "type", params: { type: "array" }, message: "must be array" };
        if (vErrors === null) {
          vErrors = [err8];
        } else {
          vErrors.push(err8);
        }
        errors++;
      }
    }
    if (data.additionalProperties !== void 0) {
      if (typeof data.additionalProperties !== "boolean") {
        const err9 = { instancePath: instancePath + "/additionalProperties", schemaPath: "#/properties/additionalProperties/type", keyword: "type", params: { type: "boolean" }, message: "must be boolean" };
        if (vErrors === null) {
          vErrors = [err9];
        } else {
          vErrors.push(err9);
        }
        errors++;
      }
    }
    if (data.items !== void 0) {
      if (!wrapper0.validate(data.items, { instancePath: instancePath + "/items", parentData: data, parentDataProperty: "items", rootData, dynamicAnchors })) {
        vErrors = vErrors === null ? wrapper0.validate.errors : vErrors.concat(wrapper0.validate.errors);
        errors = vErrors.length;
      }
    }
    if (data.enum !== void 0) {
      if (!Array.isArray(data.enum)) {
        const err10 = { instancePath: instancePath + "/enum", schemaPath: "#/properties/enum/type", keyword: "type", params: { type: "array" }, message: "must be array" };
        if (vErrors === null) {
          vErrors = [err10];
        } else {
          vErrors.push(err10);
        }
        errors++;
      }
    }
  } else {
    const err11 = { instancePath, schemaPath: "#/type", keyword: "type", params: { type: "object" }, message: "must be object" };
    if (vErrors === null) {
      vErrors = [err11];
    } else {
      vErrors.push(err11);
    }
    errors++;
  }
  validate21.errors = vErrors;
  return errors === 0;
}
validate21.evaluated = { "props": true, "dynamicProps": false, "dynamicItems": false };
var pattern42 = new RegExp("^custom_", "u");
function validate202(data, { instancePath = "", parentData, parentDataProperty, rootData = data, dynamicAnchors = {} } = {}) {
  ;
  let vErrors = null;
  let errors = 0;
  const evaluated0 = validate202.evaluated;
  if (evaluated0.dynamicProps) {
    evaluated0.props = void 0;
  }
  if (evaluated0.dynamicItems) {
    evaluated0.items = void 0;
  }
  if (data && typeof data == "object" && !Array.isArray(data)) {
    if (data.log_type !== void 0) {
      if (typeof data.log_type !== "string") {
        const err0 = { instancePath: instancePath + "/log_type", schemaPath: "#/properties/log_type/type", keyword: "type", params: { type: "string" }, message: "must be string" };
        if (vErrors === null) {
          vErrors = [err0];
        } else {
          vErrors.push(err0);
        }
        errors++;
      }
    }
    if (data.started_at !== void 0) {
      let data1 = data.started_at;
      if (typeof data1 === "string") {
        if (!formats02.validate(data1)) {
          const err1 = { instancePath: instancePath + "/started_at", schemaPath: "#/properties/started_at/format", keyword: "format", params: { format: "date-time" }, message: 'must match format "date-time"' };
          if (vErrors === null) {
            vErrors = [err1];
          } else {
            vErrors.push(err1);
          }
          errors++;
        }
      } else {
        const err2 = { instancePath: instancePath + "/started_at", schemaPath: "#/properties/started_at/type", keyword: "type", params: { type: "string" }, message: "must be string" };
        if (vErrors === null) {
          vErrors = [err2];
        } else {
          vErrors.push(err2);
        }
        errors++;
      }
    }
    if (data.ended_at !== void 0) {
      let data2 = data.ended_at;
      if (typeof data2 === "string") {
        if (!formats02.validate(data2)) {
          const err3 = { instancePath: instancePath + "/ended_at", schemaPath: "#/properties/ended_at/format", keyword: "format", params: { format: "date-time" }, message: 'must match format "date-time"' };
          if (vErrors === null) {
            vErrors = [err3];
          } else {
            vErrors.push(err3);
          }
          errors++;
        }
      } else {
        const err4 = { instancePath: instancePath + "/ended_at", schemaPath: "#/properties/ended_at/type", keyword: "type", params: { type: "string" }, message: "must be string" };
        if (vErrors === null) {
          vErrors = [err4];
        } else {
          vErrors.push(err4);
        }
        errors++;
      }
    }
    if (data.content_type !== void 0) {
      if (typeof data.content_type !== "string") {
        const err5 = { instancePath: instancePath + "/content_type", schemaPath: "#/properties/content_type/type", keyword: "type", params: { type: "string" }, message: "must be string" };
        if (vErrors === null) {
          vErrors = [err5];
        } else {
          vErrors.push(err5);
        }
        errors++;
      }
    }
    if (data.captured_at !== void 0) {
      let data4 = data.captured_at;
      if (typeof data4 === "string") {
        if (!formats02.validate(data4)) {
          const err6 = { instancePath: instancePath + "/captured_at", schemaPath: "#/properties/captured_at/format", keyword: "format", params: { format: "date-time" }, message: 'must match format "date-time"' };
          if (vErrors === null) {
            vErrors = [err6];
          } else {
            vErrors.push(err6);
          }
          errors++;
        }
      } else {
        const err7 = { instancePath: instancePath + "/captured_at", schemaPath: "#/properties/captured_at/type", keyword: "type", params: { type: "string" }, message: "must be string" };
        if (vErrors === null) {
          vErrors = [err7];
        } else {
          vErrors.push(err7);
        }
        errors++;
      }
    }
    if (data.width_px !== void 0) {
      let data5 = data.width_px;
      if (!(typeof data5 == "number" && (!(data5 % 1) && !isNaN(data5)))) {
        const err8 = { instancePath: instancePath + "/width_px", schemaPath: "#/properties/width_px/type", keyword: "type", params: { type: "integer" }, message: "must be integer" };
        if (vErrors === null) {
          vErrors = [err8];
        } else {
          vErrors.push(err8);
        }
        errors++;
      }
    }
    if (data.height_px !== void 0) {
      let data6 = data.height_px;
      if (!(typeof data6 == "number" && (!(data6 % 1) && !isNaN(data6)))) {
        const err9 = { instancePath: instancePath + "/height_px", schemaPath: "#/properties/height_px/type", keyword: "type", params: { type: "integer" }, message: "must be integer" };
        if (vErrors === null) {
          vErrors = [err9];
        } else {
          vErrors.push(err9);
        }
        errors++;
      }
    }
    if (data.commands !== void 0) {
      let data7 = data.commands;
      if (data7 && typeof data7 == "object" && !Array.isArray(data7)) {
        for (const key0 in data7) {
          const _errs18 = errors;
          if (typeof key0 === "string") {
            if (func12(key0) < 1) {
              const err10 = { instancePath: instancePath + "/commands", schemaPath: "#/properties/commands/propertyNames/minLength", keyword: "minLength", params: { limit: 1 }, message: "must NOT have fewer than 1 characters", propertyName: key0 };
              if (vErrors === null) {
                vErrors = [err10];
              } else {
                vErrors.push(err10);
              }
              errors++;
            }
          } else {
            const err11 = { instancePath: instancePath + "/commands", schemaPath: "#/properties/commands/propertyNames/type", keyword: "type", params: { type: "string" }, message: "must be string", propertyName: key0 };
            if (vErrors === null) {
              vErrors = [err11];
            } else {
              vErrors.push(err11);
            }
            errors++;
          }
          var valid1 = _errs18 === errors;
          if (!valid1) {
            const err12 = { instancePath: instancePath + "/commands", schemaPath: "#/properties/commands/propertyNames", keyword: "propertyNames", params: { propertyName: key0 }, message: "property name must be valid" };
            if (vErrors === null) {
              vErrors = [err12];
            } else {
              vErrors.push(err12);
            }
            errors++;
          }
        }
        for (const key1 in data7) {
          let data8 = data7[key1];
          if (data8 && typeof data8 == "object" && !Array.isArray(data8)) {
            if (data8.parameters_schema === void 0) {
              const err13 = { instancePath: instancePath + "/commands/" + key1.replace(/~/g, "~0").replace(/\//g, "~1"), schemaPath: "#/properties/commands/additionalProperties/required", keyword: "required", params: { missingProperty: "parameters_schema" }, message: "must have required property 'parameters_schema'" };
              if (vErrors === null) {
                vErrors = [err13];
              } else {
                vErrors.push(err13);
              }
              errors++;
            }
            if (data8.description !== void 0) {
              if (typeof data8.description !== "string") {
                const err14 = { instancePath: instancePath + "/commands/" + key1.replace(/~/g, "~0").replace(/\//g, "~1") + "/description", schemaPath: "#/properties/commands/additionalProperties/properties/description/type", keyword: "type", params: { type: "string" }, message: "must be string" };
                if (vErrors === null) {
                  vErrors = [err14];
                } else {
                  vErrors.push(err14);
                }
                errors++;
              }
            }
            if (data8.parameters_schema !== void 0) {
              if (!validate21(data8.parameters_schema, { instancePath: instancePath + "/commands/" + key1.replace(/~/g, "~0").replace(/\//g, "~1") + "/parameters_schema", parentData: data8, parentDataProperty: "parameters_schema", rootData, dynamicAnchors })) {
                vErrors = vErrors === null ? validate21.errors : vErrors.concat(validate21.errors);
                errors = vErrors.length;
              }
            }
          } else {
            const err15 = { instancePath: instancePath + "/commands/" + key1.replace(/~/g, "~0").replace(/\//g, "~1"), schemaPath: "#/properties/commands/additionalProperties/type", keyword: "type", params: { type: "object" }, message: "must be object" };
            if (vErrors === null) {
              vErrors = [err15];
            } else {
              vErrors.push(err15);
            }
            errors++;
          }
        }
      } else {
        const err16 = { instancePath: instancePath + "/commands", schemaPath: "#/properties/commands/type", keyword: "type", params: { type: "object" }, message: "must be object" };
        if (vErrors === null) {
          vErrors = [err16];
        } else {
          vErrors.push(err16);
        }
        errors++;
      }
    }
    if (data.extra !== void 0) {
      let data11 = data.extra;
      if (!(data11 && typeof data11 == "object" && !Array.isArray(data11))) {
        const err17 = { instancePath: instancePath + "/extra", schemaPath: "#/properties/extra/type", keyword: "type", params: { type: "object" }, message: "must be object" };
        if (vErrors === null) {
          vErrors = [err17];
        } else {
          vErrors.push(err17);
        }
        errors++;
      }
    }
    for (const key2 in data) {
      if (pattern42.test(key2)) {
        let data12 = data[key2];
        if (!(data12 && typeof data12 == "object" && !Array.isArray(data12))) {
          const err18 = { instancePath: instancePath + "/" + key2.replace(/~/g, "~0").replace(/\//g, "~1"), schemaPath: "#/$defs/customSection/type", keyword: "type", params: { type: "object" }, message: "must be object" };
          if (vErrors === null) {
            vErrors = [err18];
          } else {
            vErrors.push(err18);
          }
          errors++;
        }
      }
    }
  } else {
    const err19 = { instancePath, schemaPath: "#/type", keyword: "type", params: { type: "object" }, message: "must be object" };
    if (vErrors === null) {
      vErrors = [err19];
    } else {
      vErrors.push(err19);
    }
    errors++;
  }
  validate202.errors = vErrors;
  return errors === 0;
}
validate202.evaluated = { "props": true, "dynamicProps": false, "dynamicItems": false };

// ../../../../../../tmp/atlas-protocol-validators/task.mjs
var task_default = validate203;
var pattern43 = new RegExp("^custom_", "u");
var func13 = require_ucs2length().default;
function validate203(data, { instancePath = "", parentData, parentDataProperty, rootData = data, dynamicAnchors = {} } = {}) {
  ;
  let vErrors = null;
  let errors = 0;
  const evaluated0 = validate203.evaluated;
  if (evaluated0.dynamicProps) {
    evaluated0.props = void 0;
  }
  if (evaluated0.dynamicItems) {
    evaluated0.items = void 0;
  }
  if (data && typeof data == "object" && !Array.isArray(data)) {
    if (data.components === void 0) {
      const err0 = { instancePath, schemaPath: "#/required", keyword: "required", params: { missingProperty: "components" }, message: "must have required property 'components'" };
      if (vErrors === null) {
        vErrors = [err0];
      } else {
        vErrors.push(err0);
      }
      errors++;
    }
    for (const key0 in data) {
      if (!(key0 === "description" || key0 === "created_by" || key0 === "extra" || key0 === "components" || pattern43.test(key0))) {
        const err1 = { instancePath, schemaPath: "#/additionalProperties", keyword: "additionalProperties", params: { additionalProperty: key0 }, message: "must NOT have additional properties" };
        if (vErrors === null) {
          vErrors = [err1];
        } else {
          vErrors.push(err1);
        }
        errors++;
      }
    }
    if (data.description !== void 0) {
      if (typeof data.description !== "string") {
        const err2 = { instancePath: instancePath + "/description", schemaPath: "#/properties/description/type", keyword: "type", params: { type: "string" }, message: "must be string" };
        if (vErrors === null) {
          vErrors = [err2];
        } else {
          vErrors.push(err2);
        }
        errors++;
      }
    }
    if (data.created_by !== void 0) {
      if (typeof data.created_by !== "string") {
        const err3 = { instancePath: instancePath + "/created_by", schemaPath: "#/properties/created_by/type", keyword: "type", params: { type: "string" }, message: "must be string" };
        if (vErrors === null) {
          vErrors = [err3];
        } else {
          vErrors.push(err3);
        }
        errors++;
      }
    }
    if (data.extra !== void 0) {
      let data2 = data.extra;
      if (!(data2 && typeof data2 == "object" && !Array.isArray(data2))) {
        const err4 = { instancePath: instancePath + "/extra", schemaPath: "#/properties/extra/type", keyword: "type", params: { type: "object" }, message: "must be object" };
        if (vErrors === null) {
          vErrors = [err4];
        } else {
          vErrors.push(err4);
        }
        errors++;
      }
    }
    if (data.components !== void 0) {
      let data3 = data.components;
      if (data3 && typeof data3 == "object" && !Array.isArray(data3)) {
        if (data3.command === void 0) {
          const err5 = { instancePath: instancePath + "/components", schemaPath: "#/properties/components/required", keyword: "required", params: { missingProperty: "command" }, message: "must have required property 'command'" };
          if (vErrors === null) {
            vErrors = [err5];
          } else {
            vErrors.push(err5);
          }
          errors++;
        }
        if (data3.parameters === void 0) {
          const err6 = { instancePath: instancePath + "/components", schemaPath: "#/properties/components/required", keyword: "required", params: { missingProperty: "parameters" }, message: "must have required property 'parameters'" };
          if (vErrors === null) {
            vErrors = [err6];
          } else {
            vErrors.push(err6);
          }
          errors++;
        }
        for (const key1 in data3) {
          if (!(key1 === "command" || key1 === "parameters" || key1 === "progress" || key1 === "result" || key1 === "error")) {
            const err7 = { instancePath: instancePath + "/components", schemaPath: "#/properties/components/additionalProperties", keyword: "additionalProperties", params: { additionalProperty: key1 }, message: "must NOT have additional properties" };
            if (vErrors === null) {
              vErrors = [err7];
            } else {
              vErrors.push(err7);
            }
            errors++;
          }
        }
        if (data3.command !== void 0) {
          let data4 = data3.command;
          if (data4 && typeof data4 == "object" && !Array.isArray(data4)) {
            if (data4.type === void 0) {
              const err8 = { instancePath: instancePath + "/components/command", schemaPath: "#/properties/components/properties/command/required", keyword: "required", params: { missingProperty: "type" }, message: "must have required property 'type'" };
              if (vErrors === null) {
                vErrors = [err8];
              } else {
                vErrors.push(err8);
              }
              errors++;
            }
            for (const key2 in data4) {
              if (!(key2 === "type")) {
                const err9 = { instancePath: instancePath + "/components/command", schemaPath: "#/properties/components/properties/command/additionalProperties", keyword: "additionalProperties", params: { additionalProperty: key2 }, message: "must NOT have additional properties" };
                if (vErrors === null) {
                  vErrors = [err9];
                } else {
                  vErrors.push(err9);
                }
                errors++;
              }
            }
            if (data4.type !== void 0) {
              let data5 = data4.type;
              if (typeof data5 === "string") {
                if (func13(data5) < 1) {
                  const err10 = { instancePath: instancePath + "/components/command/type", schemaPath: "#/properties/components/properties/command/properties/type/minLength", keyword: "minLength", params: { limit: 1 }, message: "must NOT have fewer than 1 characters" };
                  if (vErrors === null) {
                    vErrors = [err10];
                  } else {
                    vErrors.push(err10);
                  }
                  errors++;
                }
              } else {
                const err11 = { instancePath: instancePath + "/components/command/type", schemaPath: "#/properties/components/properties/command/properties/type/type", keyword: "type", params: { type: "string" }, message: "must be string" };
                if (vErrors === null) {
                  vErrors = [err11];
                } else {
                  vErrors.push(err11);
                }
                errors++;
              }
            }
          } else {
            const err12 = { instancePath: instancePath + "/components/command", schemaPath: "#/properties/components/properties/command/type", keyword: "type", params: { type: "object" }, message: "must be object" };
            if (vErrors === null) {
              vErrors = [err12];
            } else {
              vErrors.push(err12);
            }
            errors++;
          }
        }
        if (data3.parameters !== void 0) {
          let data6 = data3.parameters;
          if (!(data6 && typeof data6 == "object" && !Array.isArray(data6))) {
            const err13 = { instancePath: instancePath + "/components/parameters", schemaPath: "#/properties/components/properties/parameters/type", keyword: "type", params: { type: "object" }, message: "must be object" };
            if (vErrors === null) {
              vErrors = [err13];
            } else {
              vErrors.push(err13);
            }
            errors++;
          }
        }
        if (data3.progress !== void 0) {
          let data7 = data3.progress;
          if (!(data7 && typeof data7 == "object" && !Array.isArray(data7))) {
            const err14 = { instancePath: instancePath + "/components/progress", schemaPath: "#/properties/components/properties/progress/type", keyword: "type", params: { type: "object" }, message: "must be object" };
            if (vErrors === null) {
              vErrors = [err14];
            } else {
              vErrors.push(err14);
            }
            errors++;
          }
        }
        if (data3.result !== void 0) {
          let data8 = data3.result;
          if (!(data8 && typeof data8 == "object" && !Array.isArray(data8))) {
            const err15 = { instancePath: instancePath + "/components/result", schemaPath: "#/properties/components/properties/result/type", keyword: "type", params: { type: "object" }, message: "must be object" };
            if (vErrors === null) {
              vErrors = [err15];
            } else {
              vErrors.push(err15);
            }
            errors++;
          }
        }
        if (data3.error !== void 0) {
          let data9 = data3.error;
          if (!(data9 && typeof data9 == "object" && !Array.isArray(data9))) {
            const err16 = { instancePath: instancePath + "/components/error", schemaPath: "#/properties/components/properties/error/type", keyword: "type", params: { type: "object" }, message: "must be object" };
            if (vErrors === null) {
              vErrors = [err16];
            } else {
              vErrors.push(err16);
            }
            errors++;
          }
        }
      } else {
        const err17 = { instancePath: instancePath + "/components", schemaPath: "#/properties/components/type", keyword: "type", params: { type: "object" }, message: "must be object" };
        if (vErrors === null) {
          vErrors = [err17];
        } else {
          vErrors.push(err17);
        }
        errors++;
      }
    }
    for (const key3 in data) {
      if (pattern43.test(key3)) {
        let data10 = data[key3];
        if (!(data10 && typeof data10 == "object" && !Array.isArray(data10))) {
          const err18 = { instancePath: instancePath + "/" + key3.replace(/~/g, "~0").replace(/\//g, "~1"), schemaPath: "#/$defs/customSection/type", keyword: "type", params: { type: "object" }, message: "must be object" };
          if (vErrors === null) {
            vErrors = [err18];
          } else {
            vErrors.push(err18);
          }
          errors++;
        }
      }
    }
  } else {
    const err19 = { instancePath, schemaPath: "#/type", keyword: "type", params: { type: "object" }, message: "must be object" };
    if (vErrors === null) {
      vErrors = [err19];
    } else {
      vErrors.push(err19);
    }
    errors++;
  }
  validate203.errors = vErrors;
  return errors === 0;
}
validate203.evaluated = { "props": true, "dynamicProps": false, "dynamicItems": false };

// ../../../../../../tmp/atlas-protocol-validators/observation.mjs
var observation_default = validate204;
var schema31 = { "$schema": "https://json-schema.org/draft/2020-12/schema", "$id": "https://atlas.local/schemas/observation.schema.json", "title": "Atlas observation JSON", "$defs": { "customSection": { "type": "object" } }, "type": "object", "required": ["state"], "properties": { "state": { "type": "string", "enum": ["active", "inactive", "ended"] }, "latest_sighting": { "type": "object", "additionalProperties": false, "required": ["observed_at", "kind", "data"], "properties": { "observed_at": { "type": "string", "format": "date-time" }, "kind": { "type": "string", "minLength": 1 }, "data": { "type": "object" }, "extra": { "type": "object" } } }, "sightings_object_id": { "type": "string" }, "extra": { "type": "object" } }, "patternProperties": { "^custom_": { "$ref": "#/$defs/customSection" } }, "additionalProperties": false };
var pattern44 = new RegExp("^custom_", "u");
var formats03 = require_formats().fullFormats["date-time"];
var func14 = require_ucs2length().default;
function validate204(data, { instancePath = "", parentData, parentDataProperty, rootData = data, dynamicAnchors = {} } = {}) {
  ;
  let vErrors = null;
  let errors = 0;
  const evaluated0 = validate204.evaluated;
  if (evaluated0.dynamicProps) {
    evaluated0.props = void 0;
  }
  if (evaluated0.dynamicItems) {
    evaluated0.items = void 0;
  }
  if (data && typeof data == "object" && !Array.isArray(data)) {
    if (data.state === void 0) {
      const err0 = { instancePath, schemaPath: "#/required", keyword: "required", params: { missingProperty: "state" }, message: "must have required property 'state'" };
      if (vErrors === null) {
        vErrors = [err0];
      } else {
        vErrors.push(err0);
      }
      errors++;
    }
    for (const key0 in data) {
      if (!(key0 === "state" || key0 === "latest_sighting" || key0 === "sightings_object_id" || key0 === "extra" || pattern44.test(key0))) {
        const err1 = { instancePath, schemaPath: "#/additionalProperties", keyword: "additionalProperties", params: { additionalProperty: key0 }, message: "must NOT have additional properties" };
        if (vErrors === null) {
          vErrors = [err1];
        } else {
          vErrors.push(err1);
        }
        errors++;
      }
    }
    if (data.state !== void 0) {
      let data0 = data.state;
      if (typeof data0 !== "string") {
        const err2 = { instancePath: instancePath + "/state", schemaPath: "#/properties/state/type", keyword: "type", params: { type: "string" }, message: "must be string" };
        if (vErrors === null) {
          vErrors = [err2];
        } else {
          vErrors.push(err2);
        }
        errors++;
      }
      if (!(data0 === "active" || data0 === "inactive" || data0 === "ended")) {
        const err3 = { instancePath: instancePath + "/state", schemaPath: "#/properties/state/enum", keyword: "enum", params: { allowedValues: schema31.properties.state.enum }, message: "must be equal to one of the allowed values" };
        if (vErrors === null) {
          vErrors = [err3];
        } else {
          vErrors.push(err3);
        }
        errors++;
      }
    }
    if (data.latest_sighting !== void 0) {
      let data1 = data.latest_sighting;
      if (data1 && typeof data1 == "object" && !Array.isArray(data1)) {
        if (data1.observed_at === void 0) {
          const err4 = { instancePath: instancePath + "/latest_sighting", schemaPath: "#/properties/latest_sighting/required", keyword: "required", params: { missingProperty: "observed_at" }, message: "must have required property 'observed_at'" };
          if (vErrors === null) {
            vErrors = [err4];
          } else {
            vErrors.push(err4);
          }
          errors++;
        }
        if (data1.kind === void 0) {
          const err5 = { instancePath: instancePath + "/latest_sighting", schemaPath: "#/properties/latest_sighting/required", keyword: "required", params: { missingProperty: "kind" }, message: "must have required property 'kind'" };
          if (vErrors === null) {
            vErrors = [err5];
          } else {
            vErrors.push(err5);
          }
          errors++;
        }
        if (data1.data === void 0) {
          const err6 = { instancePath: instancePath + "/latest_sighting", schemaPath: "#/properties/latest_sighting/required", keyword: "required", params: { missingProperty: "data" }, message: "must have required property 'data'" };
          if (vErrors === null) {
            vErrors = [err6];
          } else {
            vErrors.push(err6);
          }
          errors++;
        }
        for (const key1 in data1) {
          if (!(key1 === "observed_at" || key1 === "kind" || key1 === "data" || key1 === "extra")) {
            const err7 = { instancePath: instancePath + "/latest_sighting", schemaPath: "#/properties/latest_sighting/additionalProperties", keyword: "additionalProperties", params: { additionalProperty: key1 }, message: "must NOT have additional properties" };
            if (vErrors === null) {
              vErrors = [err7];
            } else {
              vErrors.push(err7);
            }
            errors++;
          }
        }
        if (data1.observed_at !== void 0) {
          let data2 = data1.observed_at;
          if (typeof data2 === "string") {
            if (!formats03.validate(data2)) {
              const err8 = { instancePath: instancePath + "/latest_sighting/observed_at", schemaPath: "#/properties/latest_sighting/properties/observed_at/format", keyword: "format", params: { format: "date-time" }, message: 'must match format "date-time"' };
              if (vErrors === null) {
                vErrors = [err8];
              } else {
                vErrors.push(err8);
              }
              errors++;
            }
          } else {
            const err9 = { instancePath: instancePath + "/latest_sighting/observed_at", schemaPath: "#/properties/latest_sighting/properties/observed_at/type", keyword: "type", params: { type: "string" }, message: "must be string" };
            if (vErrors === null) {
              vErrors = [err9];
            } else {
              vErrors.push(err9);
            }
            errors++;
          }
        }
        if (data1.kind !== void 0) {
          let data3 = data1.kind;
          if (typeof data3 === "string") {
            if (func14(data3) < 1) {
              const err10 = { instancePath: instancePath + "/latest_sighting/kind", schemaPath: "#/properties/latest_sighting/properties/kind/minLength", keyword: "minLength", params: { limit: 1 }, message: "must NOT have fewer than 1 characters" };
              if (vErrors === null) {
                vErrors = [err10];
              } else {
                vErrors.push(err10);
              }
              errors++;
            }
          } else {
            const err11 = { instancePath: instancePath + "/latest_sighting/kind", schemaPath: "#/properties/latest_sighting/properties/kind/type", keyword: "type", params: { type: "string" }, message: "must be string" };
            if (vErrors === null) {
              vErrors = [err11];
            } else {
              vErrors.push(err11);
            }
            errors++;
          }
        }
        if (data1.data !== void 0) {
          let data4 = data1.data;
          if (!(data4 && typeof data4 == "object" && !Array.isArray(data4))) {
            const err12 = { instancePath: instancePath + "/latest_sighting/data", schemaPath: "#/properties/latest_sighting/properties/data/type", keyword: "type", params: { type: "object" }, message: "must be object" };
            if (vErrors === null) {
              vErrors = [err12];
            } else {
              vErrors.push(err12);
            }
            errors++;
          }
        }
        if (data1.extra !== void 0) {
          let data5 = data1.extra;
          if (!(data5 && typeof data5 == "object" && !Array.isArray(data5))) {
            const err13 = { instancePath: instancePath + "/latest_sighting/extra", schemaPath: "#/properties/latest_sighting/properties/extra/type", keyword: "type", params: { type: "object" }, message: "must be object" };
            if (vErrors === null) {
              vErrors = [err13];
            } else {
              vErrors.push(err13);
            }
            errors++;
          }
        }
      } else {
        const err14 = { instancePath: instancePath + "/latest_sighting", schemaPath: "#/properties/latest_sighting/type", keyword: "type", params: { type: "object" }, message: "must be object" };
        if (vErrors === null) {
          vErrors = [err14];
        } else {
          vErrors.push(err14);
        }
        errors++;
      }
    }
    if (data.sightings_object_id !== void 0) {
      if (typeof data.sightings_object_id !== "string") {
        const err15 = { instancePath: instancePath + "/sightings_object_id", schemaPath: "#/properties/sightings_object_id/type", keyword: "type", params: { type: "string" }, message: "must be string" };
        if (vErrors === null) {
          vErrors = [err15];
        } else {
          vErrors.push(err15);
        }
        errors++;
      }
    }
    if (data.extra !== void 0) {
      let data7 = data.extra;
      if (!(data7 && typeof data7 == "object" && !Array.isArray(data7))) {
        const err16 = { instancePath: instancePath + "/extra", schemaPath: "#/properties/extra/type", keyword: "type", params: { type: "object" }, message: "must be object" };
        if (vErrors === null) {
          vErrors = [err16];
        } else {
          vErrors.push(err16);
        }
        errors++;
      }
    }
    for (const key2 in data) {
      if (pattern44.test(key2)) {
        let data8 = data[key2];
        if (!(data8 && typeof data8 == "object" && !Array.isArray(data8))) {
          const err17 = { instancePath: instancePath + "/" + key2.replace(/~/g, "~0").replace(/\//g, "~1"), schemaPath: "#/$defs/customSection/type", keyword: "type", params: { type: "object" }, message: "must be object" };
          if (vErrors === null) {
            vErrors = [err17];
          } else {
            vErrors.push(err17);
          }
          errors++;
        }
      }
    }
  } else {
    const err18 = { instancePath, schemaPath: "#/type", keyword: "type", params: { type: "object" }, message: "must be object" };
    if (vErrors === null) {
      vErrors = [err18];
    } else {
      vErrors.push(err18);
    }
    errors++;
  }
  validate204.errors = vErrors;
  return errors === 0;
}
validate204.evaluated = { "props": true, "dynamicProps": false, "dynamicItems": false };

// ../../../../../../tmp/atlas-protocol-validators/command-catalog.mjs
var command_catalog_default = validate205;
var func15 = require_ucs2length().default;
var schema322 = { "type": "object", "required": ["type"], "additionalProperties": false, "properties": { "type": { "type": "string", "enum": ["object", "array", "string", "number", "integer", "boolean"] }, "properties": { "type": "object", "additionalProperties": { "$ref": "#/$defs/parameterSchema" } }, "required": { "type": "array", "items": { "type": "string", "minLength": 1 }, "uniqueItems": true }, "additionalProperties": { "type": "boolean" }, "items": { "$ref": "#/$defs/parameterSchema" }, "enum": { "type": "array", "items": { "type": ["string", "number", "integer", "boolean", "object", "array", "null"] } } } };
var wrapper02 = { validate: validate212 };
function validate212(data, { instancePath = "", parentData, parentDataProperty, rootData = data, dynamicAnchors = {} } = {}) {
  let vErrors = null;
  let errors = 0;
  const evaluated0 = validate212.evaluated;
  if (evaluated0.dynamicProps) {
    evaluated0.props = void 0;
  }
  if (evaluated0.dynamicItems) {
    evaluated0.items = void 0;
  }
  if (data && typeof data == "object" && !Array.isArray(data)) {
    if (data.type === void 0) {
      const err0 = { instancePath, schemaPath: "#/required", keyword: "required", params: { missingProperty: "type" }, message: "must have required property 'type'" };
      if (vErrors === null) {
        vErrors = [err0];
      } else {
        vErrors.push(err0);
      }
      errors++;
    }
    for (const key0 in data) {
      if (!(key0 === "type" || key0 === "properties" || key0 === "required" || key0 === "additionalProperties" || key0 === "items" || key0 === "enum")) {
        const err1 = { instancePath, schemaPath: "#/additionalProperties", keyword: "additionalProperties", params: { additionalProperty: key0 }, message: "must NOT have additional properties" };
        if (vErrors === null) {
          vErrors = [err1];
        } else {
          vErrors.push(err1);
        }
        errors++;
      }
    }
    if (data.type !== void 0) {
      let data0 = data.type;
      if (typeof data0 !== "string") {
        const err2 = { instancePath: instancePath + "/type", schemaPath: "#/properties/type/type", keyword: "type", params: { type: "string" }, message: "must be string" };
        if (vErrors === null) {
          vErrors = [err2];
        } else {
          vErrors.push(err2);
        }
        errors++;
      }
      if (!(data0 === "object" || data0 === "array" || data0 === "string" || data0 === "number" || data0 === "integer" || data0 === "boolean")) {
        const err3 = { instancePath: instancePath + "/type", schemaPath: "#/properties/type/enum", keyword: "enum", params: { allowedValues: schema322.properties.type.enum }, message: "must be equal to one of the allowed values" };
        if (vErrors === null) {
          vErrors = [err3];
        } else {
          vErrors.push(err3);
        }
        errors++;
      }
    }
    if (data.properties !== void 0) {
      let data1 = data.properties;
      if (data1 && typeof data1 == "object" && !Array.isArray(data1)) {
        for (const key1 in data1) {
          if (!wrapper02.validate(data1[key1], { instancePath: instancePath + "/properties/" + key1.replace(/~/g, "~0").replace(/\//g, "~1"), parentData: data1, parentDataProperty: key1, rootData, dynamicAnchors })) {
            vErrors = vErrors === null ? wrapper02.validate.errors : vErrors.concat(wrapper02.validate.errors);
            errors = vErrors.length;
          }
        }
      } else {
        const err4 = { instancePath: instancePath + "/properties", schemaPath: "#/properties/properties/type", keyword: "type", params: { type: "object" }, message: "must be object" };
        if (vErrors === null) {
          vErrors = [err4];
        } else {
          vErrors.push(err4);
        }
        errors++;
      }
    }
    if (data.required !== void 0) {
      let data3 = data.required;
      if (Array.isArray(data3)) {
        const len0 = data3.length;
        for (let i0 = 0; i0 < len0; i0++) {
          let data4 = data3[i0];
          if (typeof data4 === "string") {
            if (func15(data4) < 1) {
              const err5 = { instancePath: instancePath + "/required/" + i0, schemaPath: "#/properties/required/items/minLength", keyword: "minLength", params: { limit: 1 }, message: "must NOT have fewer than 1 characters" };
              if (vErrors === null) {
                vErrors = [err5];
              } else {
                vErrors.push(err5);
              }
              errors++;
            }
          } else {
            const err6 = { instancePath: instancePath + "/required/" + i0, schemaPath: "#/properties/required/items/type", keyword: "type", params: { type: "string" }, message: "must be string" };
            if (vErrors === null) {
              vErrors = [err6];
            } else {
              vErrors.push(err6);
            }
            errors++;
          }
        }
        let i1 = data3.length;
        let j0;
        if (i1 > 1) {
          const indices0 = {};
          for (; i1--; ) {
            let item0 = data3[i1];
            if (typeof item0 !== "string") {
              continue;
            }
            if (typeof indices0[item0] == "number") {
              j0 = indices0[item0];
              const err7 = { instancePath: instancePath + "/required", schemaPath: "#/properties/required/uniqueItems", keyword: "uniqueItems", params: { i: i1, j: j0 }, message: "must NOT have duplicate items (items ## " + j0 + " and " + i1 + " are identical)" };
              if (vErrors === null) {
                vErrors = [err7];
              } else {
                vErrors.push(err7);
              }
              errors++;
              break;
            }
            indices0[item0] = i1;
          }
        }
      } else {
        const err8 = { instancePath: instancePath + "/required", schemaPath: "#/properties/required/type", keyword: "type", params: { type: "array" }, message: "must be array" };
        if (vErrors === null) {
          vErrors = [err8];
        } else {
          vErrors.push(err8);
        }
        errors++;
      }
    }
    if (data.additionalProperties !== void 0) {
      if (typeof data.additionalProperties !== "boolean") {
        const err9 = { instancePath: instancePath + "/additionalProperties", schemaPath: "#/properties/additionalProperties/type", keyword: "type", params: { type: "boolean" }, message: "must be boolean" };
        if (vErrors === null) {
          vErrors = [err9];
        } else {
          vErrors.push(err9);
        }
        errors++;
      }
    }
    if (data.items !== void 0) {
      if (!wrapper02.validate(data.items, { instancePath: instancePath + "/items", parentData: data, parentDataProperty: "items", rootData, dynamicAnchors })) {
        vErrors = vErrors === null ? wrapper02.validate.errors : vErrors.concat(wrapper02.validate.errors);
        errors = vErrors.length;
      }
    }
    if (data.enum !== void 0) {
      let data7 = data.enum;
      if (Array.isArray(data7)) {
        const len1 = data7.length;
        for (let i2 = 0; i2 < len1; i2++) {
          let data8 = data7[i2];
          if (typeof data8 != "object" && typeof data8 !== "string" && !(typeof data8 == "number") && typeof data8 !== "boolean") {
            const err10 = { instancePath: instancePath + "/enum/" + i2, schemaPath: "#/properties/enum/items/type", keyword: "type", params: { type: schema322.properties.enum.items.type }, message: "must be string,number,integer,boolean,object,array,null" };
            if (vErrors === null) {
              vErrors = [err10];
            } else {
              vErrors.push(err10);
            }
            errors++;
          }
        }
      } else {
        const err11 = { instancePath: instancePath + "/enum", schemaPath: "#/properties/enum/type", keyword: "type", params: { type: "array" }, message: "must be array" };
        if (vErrors === null) {
          vErrors = [err11];
        } else {
          vErrors.push(err11);
        }
        errors++;
      }
    }
  } else {
    const err12 = { instancePath, schemaPath: "#/type", keyword: "type", params: { type: "object" }, message: "must be object" };
    if (vErrors === null) {
      vErrors = [err12];
    } else {
      vErrors.push(err12);
    }
    errors++;
  }
  validate212.errors = vErrors;
  return errors === 0;
}
validate212.evaluated = { "props": true, "dynamicProps": false, "dynamicItems": false };
function validate205(data, { instancePath = "", parentData, parentDataProperty, rootData = data, dynamicAnchors = {} } = {}) {
  ;
  let vErrors = null;
  let errors = 0;
  const evaluated0 = validate205.evaluated;
  if (evaluated0.dynamicProps) {
    evaluated0.props = void 0;
  }
  if (evaluated0.dynamicItems) {
    evaluated0.items = void 0;
  }
  if (data && typeof data == "object" && !Array.isArray(data)) {
    if (data.commands === void 0) {
      const err0 = { instancePath, schemaPath: "#/required", keyword: "required", params: { missingProperty: "commands" }, message: "must have required property 'commands'" };
      if (vErrors === null) {
        vErrors = [err0];
      } else {
        vErrors.push(err0);
      }
      errors++;
    }
    for (const key0 in data) {
      if (!(key0 === "commands")) {
        const err1 = { instancePath, schemaPath: "#/additionalProperties", keyword: "additionalProperties", params: { additionalProperty: key0 }, message: "must NOT have additional properties" };
        if (vErrors === null) {
          vErrors = [err1];
        } else {
          vErrors.push(err1);
        }
        errors++;
      }
    }
    if (data.commands !== void 0) {
      let data0 = data.commands;
      if (data0 && typeof data0 == "object" && !Array.isArray(data0)) {
        for (const key1 in data0) {
          const _errs4 = errors;
          if (typeof key1 === "string") {
            if (func15(key1) < 1) {
              const err2 = { instancePath: instancePath + "/commands", schemaPath: "#/properties/commands/propertyNames/minLength", keyword: "minLength", params: { limit: 1 }, message: "must NOT have fewer than 1 characters", propertyName: key1 };
              if (vErrors === null) {
                vErrors = [err2];
              } else {
                vErrors.push(err2);
              }
              errors++;
            }
          } else {
            const err3 = { instancePath: instancePath + "/commands", schemaPath: "#/properties/commands/propertyNames/type", keyword: "type", params: { type: "string" }, message: "must be string", propertyName: key1 };
            if (vErrors === null) {
              vErrors = [err3];
            } else {
              vErrors.push(err3);
            }
            errors++;
          }
          var valid1 = _errs4 === errors;
          if (!valid1) {
            const err4 = { instancePath: instancePath + "/commands", schemaPath: "#/properties/commands/propertyNames", keyword: "propertyNames", params: { propertyName: key1 }, message: "property name must be valid" };
            if (vErrors === null) {
              vErrors = [err4];
            } else {
              vErrors.push(err4);
            }
            errors++;
          }
        }
        for (const key2 in data0) {
          let data1 = data0[key2];
          if (data1 && typeof data1 == "object" && !Array.isArray(data1)) {
            if (data1.parameters_schema === void 0) {
              const err5 = { instancePath: instancePath + "/commands/" + key2.replace(/~/g, "~0").replace(/\//g, "~1"), schemaPath: "#/properties/commands/additionalProperties/required", keyword: "required", params: { missingProperty: "parameters_schema" }, message: "must have required property 'parameters_schema'" };
              if (vErrors === null) {
                vErrors = [err5];
              } else {
                vErrors.push(err5);
              }
              errors++;
            }
            if (data1.description !== void 0) {
              if (typeof data1.description !== "string") {
                const err6 = { instancePath: instancePath + "/commands/" + key2.replace(/~/g, "~0").replace(/\//g, "~1") + "/description", schemaPath: "#/properties/commands/additionalProperties/properties/description/type", keyword: "type", params: { type: "string" }, message: "must be string" };
                if (vErrors === null) {
                  vErrors = [err6];
                } else {
                  vErrors.push(err6);
                }
                errors++;
              }
            }
            if (data1.parameters_schema !== void 0) {
              if (!validate212(data1.parameters_schema, { instancePath: instancePath + "/commands/" + key2.replace(/~/g, "~0").replace(/\//g, "~1") + "/parameters_schema", parentData: data1, parentDataProperty: "parameters_schema", rootData, dynamicAnchors })) {
                vErrors = vErrors === null ? validate212.errors : vErrors.concat(validate212.errors);
                errors = vErrors.length;
              }
            }
          } else {
            const err7 = { instancePath: instancePath + "/commands/" + key2.replace(/~/g, "~0").replace(/\//g, "~1"), schemaPath: "#/properties/commands/additionalProperties/type", keyword: "type", params: { type: "object" }, message: "must be object" };
            if (vErrors === null) {
              vErrors = [err7];
            } else {
              vErrors.push(err7);
            }
            errors++;
          }
        }
      } else {
        const err8 = { instancePath: instancePath + "/commands", schemaPath: "#/properties/commands/type", keyword: "type", params: { type: "object" }, message: "must be object" };
        if (vErrors === null) {
          vErrors = [err8];
        } else {
          vErrors.push(err8);
        }
        errors++;
      }
    }
  } else {
    const err9 = { instancePath, schemaPath: "#/type", keyword: "type", params: { type: "object" }, message: "must be object" };
    if (vErrors === null) {
      vErrors = [err9];
    } else {
      vErrors.push(err9);
    }
    errors++;
  }
  validate205.errors = vErrors;
  return errors === 0;
}
validate205.evaluated = { "props": true, "dynamicProps": false, "dynamicItems": false };

// ../../../../../../tmp/atlas-protocol-validators/change-event.mjs
var change_event_default = validate206;
var schema312 = { "$schema": "https://json-schema.org/draft/2020-12/schema", "$id": "https://atlas.local/schemas/change-event.schema.json", "title": "Atlas change event", "type": "object", "additionalProperties": false, "required": ["resource_kind", "operation", "resource_id", "occurred_at"], "properties": { "resource_kind": { "type": "string", "enum": ["entity", "object", "task", "observation"] }, "operation": { "type": "string", "enum": ["create", "update", "delete", "manifest_sync"] }, "resource_id": { "type": "string", "minLength": 1 }, "occurred_at": { "type": "string", "format": "date-time" }, "metadata": { "type": "object" } } };
var func16 = require_ucs2length().default;
var formats04 = require_formats().fullFormats["date-time"];
function validate206(data, { instancePath = "", parentData, parentDataProperty, rootData = data, dynamicAnchors = {} } = {}) {
  ;
  let vErrors = null;
  let errors = 0;
  const evaluated0 = validate206.evaluated;
  if (evaluated0.dynamicProps) {
    evaluated0.props = void 0;
  }
  if (evaluated0.dynamicItems) {
    evaluated0.items = void 0;
  }
  if (data && typeof data == "object" && !Array.isArray(data)) {
    if (data.resource_kind === void 0) {
      const err0 = { instancePath, schemaPath: "#/required", keyword: "required", params: { missingProperty: "resource_kind" }, message: "must have required property 'resource_kind'" };
      if (vErrors === null) {
        vErrors = [err0];
      } else {
        vErrors.push(err0);
      }
      errors++;
    }
    if (data.operation === void 0) {
      const err1 = { instancePath, schemaPath: "#/required", keyword: "required", params: { missingProperty: "operation" }, message: "must have required property 'operation'" };
      if (vErrors === null) {
        vErrors = [err1];
      } else {
        vErrors.push(err1);
      }
      errors++;
    }
    if (data.resource_id === void 0) {
      const err2 = { instancePath, schemaPath: "#/required", keyword: "required", params: { missingProperty: "resource_id" }, message: "must have required property 'resource_id'" };
      if (vErrors === null) {
        vErrors = [err2];
      } else {
        vErrors.push(err2);
      }
      errors++;
    }
    if (data.occurred_at === void 0) {
      const err3 = { instancePath, schemaPath: "#/required", keyword: "required", params: { missingProperty: "occurred_at" }, message: "must have required property 'occurred_at'" };
      if (vErrors === null) {
        vErrors = [err3];
      } else {
        vErrors.push(err3);
      }
      errors++;
    }
    for (const key0 in data) {
      if (!(key0 === "resource_kind" || key0 === "operation" || key0 === "resource_id" || key0 === "occurred_at" || key0 === "metadata")) {
        const err4 = { instancePath, schemaPath: "#/additionalProperties", keyword: "additionalProperties", params: { additionalProperty: key0 }, message: "must NOT have additional properties" };
        if (vErrors === null) {
          vErrors = [err4];
        } else {
          vErrors.push(err4);
        }
        errors++;
      }
    }
    if (data.resource_kind !== void 0) {
      let data0 = data.resource_kind;
      if (typeof data0 !== "string") {
        const err5 = { instancePath: instancePath + "/resource_kind", schemaPath: "#/properties/resource_kind/type", keyword: "type", params: { type: "string" }, message: "must be string" };
        if (vErrors === null) {
          vErrors = [err5];
        } else {
          vErrors.push(err5);
        }
        errors++;
      }
      if (!(data0 === "entity" || data0 === "object" || data0 === "task" || data0 === "observation")) {
        const err6 = { instancePath: instancePath + "/resource_kind", schemaPath: "#/properties/resource_kind/enum", keyword: "enum", params: { allowedValues: schema312.properties.resource_kind.enum }, message: "must be equal to one of the allowed values" };
        if (vErrors === null) {
          vErrors = [err6];
        } else {
          vErrors.push(err6);
        }
        errors++;
      }
    }
    if (data.operation !== void 0) {
      let data1 = data.operation;
      if (typeof data1 !== "string") {
        const err7 = { instancePath: instancePath + "/operation", schemaPath: "#/properties/operation/type", keyword: "type", params: { type: "string" }, message: "must be string" };
        if (vErrors === null) {
          vErrors = [err7];
        } else {
          vErrors.push(err7);
        }
        errors++;
      }
      if (!(data1 === "create" || data1 === "update" || data1 === "delete" || data1 === "manifest_sync")) {
        const err8 = { instancePath: instancePath + "/operation", schemaPath: "#/properties/operation/enum", keyword: "enum", params: { allowedValues: schema312.properties.operation.enum }, message: "must be equal to one of the allowed values" };
        if (vErrors === null) {
          vErrors = [err8];
        } else {
          vErrors.push(err8);
        }
        errors++;
      }
    }
    if (data.resource_id !== void 0) {
      let data2 = data.resource_id;
      if (typeof data2 === "string") {
        if (func16(data2) < 1) {
          const err9 = { instancePath: instancePath + "/resource_id", schemaPath: "#/properties/resource_id/minLength", keyword: "minLength", params: { limit: 1 }, message: "must NOT have fewer than 1 characters" };
          if (vErrors === null) {
            vErrors = [err9];
          } else {
            vErrors.push(err9);
          }
          errors++;
        }
      } else {
        const err10 = { instancePath: instancePath + "/resource_id", schemaPath: "#/properties/resource_id/type", keyword: "type", params: { type: "string" }, message: "must be string" };
        if (vErrors === null) {
          vErrors = [err10];
        } else {
          vErrors.push(err10);
        }
        errors++;
      }
    }
    if (data.occurred_at !== void 0) {
      let data3 = data.occurred_at;
      if (typeof data3 === "string") {
        if (!formats04.validate(data3)) {
          const err11 = { instancePath: instancePath + "/occurred_at", schemaPath: "#/properties/occurred_at/format", keyword: "format", params: { format: "date-time" }, message: 'must match format "date-time"' };
          if (vErrors === null) {
            vErrors = [err11];
          } else {
            vErrors.push(err11);
          }
          errors++;
        }
      } else {
        const err12 = { instancePath: instancePath + "/occurred_at", schemaPath: "#/properties/occurred_at/type", keyword: "type", params: { type: "string" }, message: "must be string" };
        if (vErrors === null) {
          vErrors = [err12];
        } else {
          vErrors.push(err12);
        }
        errors++;
      }
    }
    if (data.metadata !== void 0) {
      let data4 = data.metadata;
      if (!(data4 && typeof data4 == "object" && !Array.isArray(data4))) {
        const err13 = { instancePath: instancePath + "/metadata", schemaPath: "#/properties/metadata/type", keyword: "type", params: { type: "object" }, message: "must be object" };
        if (vErrors === null) {
          vErrors = [err13];
        } else {
          vErrors.push(err13);
        }
        errors++;
      }
    }
  } else {
    const err14 = { instancePath, schemaPath: "#/type", keyword: "type", params: { type: "object" }, message: "must be object" };
    if (vErrors === null) {
      vErrors = [err14];
    } else {
      vErrors.push(err14);
    }
    errors++;
  }
  validate206.errors = vErrors;
  return errors === 0;
}
validate206.evaluated = { "props": true, "dynamicProps": false, "dynamicItems": false };

// ../../../../../../tmp/atlas-protocol-validators/validation-error.mjs
var validation_error_default = validate207;
var func17 = require_ucs2length().default;
function validate207(data, { instancePath = "", parentData, parentDataProperty, rootData = data, dynamicAnchors = {} } = {}) {
  ;
  let vErrors = null;
  let errors = 0;
  const evaluated0 = validate207.evaluated;
  if (evaluated0.dynamicProps) {
    evaluated0.props = void 0;
  }
  if (evaluated0.dynamicItems) {
    evaluated0.items = void 0;
  }
  if (data && typeof data == "object" && !Array.isArray(data)) {
    if (data.field === void 0) {
      const err0 = { instancePath, schemaPath: "#/required", keyword: "required", params: { missingProperty: "field" }, message: "must have required property 'field'" };
      if (vErrors === null) {
        vErrors = [err0];
      } else {
        vErrors.push(err0);
      }
      errors++;
    }
    if (data.code === void 0) {
      const err1 = { instancePath, schemaPath: "#/required", keyword: "required", params: { missingProperty: "code" }, message: "must have required property 'code'" };
      if (vErrors === null) {
        vErrors = [err1];
      } else {
        vErrors.push(err1);
      }
      errors++;
    }
    if (data.message === void 0) {
      const err2 = { instancePath, schemaPath: "#/required", keyword: "required", params: { missingProperty: "message" }, message: "must have required property 'message'" };
      if (vErrors === null) {
        vErrors = [err2];
      } else {
        vErrors.push(err2);
      }
      errors++;
    }
    for (const key0 in data) {
      if (!(key0 === "field" || key0 === "code" || key0 === "message")) {
        const err3 = { instancePath, schemaPath: "#/additionalProperties", keyword: "additionalProperties", params: { additionalProperty: key0 }, message: "must NOT have additional properties" };
        if (vErrors === null) {
          vErrors = [err3];
        } else {
          vErrors.push(err3);
        }
        errors++;
      }
    }
    if (data.field !== void 0) {
      let data0 = data.field;
      if (typeof data0 === "string") {
        if (func17(data0) < 1) {
          const err4 = { instancePath: instancePath + "/field", schemaPath: "#/properties/field/minLength", keyword: "minLength", params: { limit: 1 }, message: "must NOT have fewer than 1 characters" };
          if (vErrors === null) {
            vErrors = [err4];
          } else {
            vErrors.push(err4);
          }
          errors++;
        }
      } else {
        const err5 = { instancePath: instancePath + "/field", schemaPath: "#/properties/field/type", keyword: "type", params: { type: "string" }, message: "must be string" };
        if (vErrors === null) {
          vErrors = [err5];
        } else {
          vErrors.push(err5);
        }
        errors++;
      }
    }
    if (data.code !== void 0) {
      let data1 = data.code;
      if (typeof data1 === "string") {
        if (func17(data1) < 1) {
          const err6 = { instancePath: instancePath + "/code", schemaPath: "#/properties/code/minLength", keyword: "minLength", params: { limit: 1 }, message: "must NOT have fewer than 1 characters" };
          if (vErrors === null) {
            vErrors = [err6];
          } else {
            vErrors.push(err6);
          }
          errors++;
        }
      } else {
        const err7 = { instancePath: instancePath + "/code", schemaPath: "#/properties/code/type", keyword: "type", params: { type: "string" }, message: "must be string" };
        if (vErrors === null) {
          vErrors = [err7];
        } else {
          vErrors.push(err7);
        }
        errors++;
      }
    }
    if (data.message !== void 0) {
      let data2 = data.message;
      if (typeof data2 === "string") {
        if (func17(data2) < 1) {
          const err8 = { instancePath: instancePath + "/message", schemaPath: "#/properties/message/minLength", keyword: "minLength", params: { limit: 1 }, message: "must NOT have fewer than 1 characters" };
          if (vErrors === null) {
            vErrors = [err8];
          } else {
            vErrors.push(err8);
          }
          errors++;
        }
      } else {
        const err9 = { instancePath: instancePath + "/message", schemaPath: "#/properties/message/type", keyword: "type", params: { type: "string" }, message: "must be string" };
        if (vErrors === null) {
          vErrors = [err9];
        } else {
          vErrors.push(err9);
        }
        errors++;
      }
    }
  } else {
    const err10 = { instancePath, schemaPath: "#/type", keyword: "type", params: { type: "object" }, message: "must be object" };
    if (vErrors === null) {
      vErrors = [err10];
    } else {
      vErrors.push(err10);
    }
    errors++;
  }
  validate207.errors = vErrors;
  return errors === 0;
}
validate207.evaluated = { "props": true, "dynamicProps": false, "dynamicItems": false };

// ../../../../../../tmp/atlas-protocol-validators/index.mjs
var validators = {
  "entity": entity_default,
  "object": object_default,
  "task": task_default,
  "observation": observation_default,
  "command-catalog": command_catalog_default,
  "change-event": change_event_default,
  "validation-error": validation_error_default
};
var index_default = validators;
export {
  index_default as default,
  validators
};
