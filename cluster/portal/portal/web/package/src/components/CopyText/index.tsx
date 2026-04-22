import { Tooltip } from "@mantine/core";
import { AnimatePresence, motion } from "framer-motion";
import { useState } from "react";
import { FaCheckDouble } from "react-icons/fa6";
import { MdOutlineContentCopy } from "react-icons/md";
import truncate from "truncate-utf8-bytes";

const CopyText = (props: { value?: string; truncate?: number }) => {
  const [copied, setCopied] = useState(false);
  const { value } = props;

  if (!value) return null;

  const display =
    props.truncate && props.truncate < value.length
      ? `${truncate(value, props.truncate)}...`
      : value;

  return (
    <span style={{ display: "inline-flex", alignItems: "center", gap: 4 }}>
      <span style={{ fontFamily: "monospace", fontSize: "0.85em" }}>
        {display}
      </span>
      <Tooltip label={copied ? "Copied!" : "Copy"} withArrow position="top">
        <button
          aria-label="Copy to clipboard"
          onClick={() => {
            navigator.clipboard.writeText(value);
            setCopied(true);
            setTimeout(() => setCopied(false), 1500);
          }}
          style={{
            display: "inline-flex",
            alignItems: "center",
            justifyContent: "center",
            background: "none",
            border: "none",
            cursor: "pointer",
            padding: 2,
            color: copied
              ? "var(--mantine-color-teal-6)"
              : "var(--mantine-color-dimmed)",
            transition: "color 200ms ease",
            borderRadius: 4,
            lineHeight: 1,
          }}
        >
          <AnimatePresence initial={false} mode="popLayout">
            <motion.div
              key={copied ? "1" : "2"}
              initial={{ y: 6, opacity: 0 }}
              animate={{ y: 0, opacity: 1 }}
              exit={{ y: -6, opacity: 0 }}
              transition={{ duration: 0.15 }}
              style={{ display: "flex" }}
            >
              {copied ? (
                <FaCheckDouble size={11} />
              ) : (
                <MdOutlineContentCopy size={12} />
              )}
            </motion.div>
          </AnimatePresence>
        </button>
      </Tooltip>
    </span>
  );
};

export default CopyText;
