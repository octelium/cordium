type QueryRoot = Document | ShadowRoot | Element;

const getShadowRoot = (element: Element): ShadowRoot | null => {
  try {
    return element.shadowRoot;
  } catch {
    return null;
  }
};

export const findElementBySelector = (
  selector: string,
  root: QueryRoot = document,
): Element | null => {
  const element = root.querySelector(selector);
  if (element) return element;

  for (const child of root.querySelectorAll("*")) {
    const shadowRoot = getShadowRoot(child);
    if (!shadowRoot) continue;
    const match = findElementBySelector(selector, shadowRoot);
    if (match) return match;
  }

  return null;
};

export const findElementsBySelector = (
  selector: string,
  root: QueryRoot = document,
): Element[] => {
  const elements = Array.from(root.querySelectorAll(selector));

  for (const child of root.querySelectorAll("*")) {
    const shadowRoot = getShadowRoot(child);
    if (shadowRoot) {
      elements.push(...findElementsBySelector(selector, shadowRoot));
    }
  }

  return elements;
};

export const getRootElement = (
  targetElement?: Element | null,
): Document | ShadowRoot => {
  const root = targetElement?.getRootNode();
  return root instanceof ShadowRoot || root instanceof Document ? root : document;
};
