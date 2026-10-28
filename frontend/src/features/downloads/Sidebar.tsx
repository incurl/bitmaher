/**
 * The sidebar of the window.
 *
 * The four categories of downloads, with the count of each one, and
 * the button that adds a download.
 */

import { DOWNLOAD_CATEGORIES, countByCategory } from './format';
import type { DownloadCategory } from './format';
import type { Download } from '../../bindings';

interface SidebarProps {
  /** The category that is shown. */
  category: DownloadCategory;
  /** Changes the category. */
  onCategoryChange: (category: DownloadCategory) => void;
  /** The downloads, for the counts. */
  downloads: Download[];
  /** Opens the add-task dialog. */
  onAdd: () => void;
  /** Removes the finished downloads. */
  onPurge: () => void;
  /** Whether the purge button is offered. */
  canPurge: boolean;
}

/**
 * The category sidebar.
 */
export function Sidebar({
  category,
  onCategoryChange,
  downloads,
  onAdd,
  onPurge,
  canPurge,
}: SidebarProps) {
  const counts = countByCategory(downloads);

  return (
    <aside className="sidebar">
      <button type="button" className="btn btn-primary sidebar-add" onClick={onAdd}>
        Add download
      </button>

      <nav className="sidebar-nav" aria-label="Categories">
        {DOWNLOAD_CATEGORIES.map((item) => {
          const count = counts[item.value];
          const active = item.value === category;

          return (
            <button
              key={item.value}
              type="button"
              className={`sidebar-item${active ? ' sidebar-item-active' : ''}`}
              onClick={() => onCategoryChange(item.value)}
              aria-current={active ? 'page' : undefined}
            >
              <span className="sidebar-label">{item.label}</span>
              <span className="sidebar-count">{count}</span>
            </button>
          );
        })}
      </nav>

      <div className="sidebar-foot">
        <button
          type="button"
          className="btn btn-sm"
          onClick={onPurge}
          disabled={!canPurge}
          title="Remove the finished downloads"
        >
          Clear finished
        </button>
      </div>
    </aside>
  );
}
