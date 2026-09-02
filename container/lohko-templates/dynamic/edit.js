import { RichText, useBlockProps } from "@wordpress/block-editor";
import "./editor.css";

/**
 * The edit function describes what the block editor renders for this
 * block. Replace this with your own markup and controls as the block
 * grows.
 *
 * @see https://developer.wordpress.org/block-editor/reference-guides/block-api/block-edit-save/#edit
 */
export default function Edit({ attributes, setAttributes }) {
  return (
    <RichText
      {...useBlockProps()}
      tagName="p"
      value={attributes.content}
      onChange={(content) => setAttributes({ content })}
      placeholder="Write something…"
    />
  );
}
