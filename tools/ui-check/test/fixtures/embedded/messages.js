console.error("Embedded script notice");
console.error("Embedded script notice");
const embeddedParameters = new URLSearchParams(location.search);
if (embeddedParameters.has("different")) console.error("Another embedded notice");
if (embeddedParameters.has("uncaught")) throw new Error("Embedded script notice");
if (embeddedParameters.has("rejection")) Promise.reject(new Error("Embedded script notice"));
