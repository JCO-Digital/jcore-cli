import { RichText, useBlockProps } from "@wordpress/block-editor";

/**
 * The save function defines the markup saved into post_content. Keep it in
 * sync with what edit() renders, or the block will be flagged invalid in
 * the editor.
 *
 * @see https://developer.wordpress.org/block-editor/reference-guides/block-api/block-edit-save/#save
 */
export default function save({ attributes }) {
  return (
    <RichText.Content
      {...useBlockProps.save()}
      tagName="p"
      value={attributes.content}
    />
  );
}
