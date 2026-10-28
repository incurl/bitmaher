/**
 * The Tauri-compatible dialog API.
 *
 * The Wails runtime has the same three dialogs, under a different
 * shape: the options are a record with capitalised names, and the
 * message dialog answers with the label of the button that was pressed.
 */
import { Dialogs } from '@wailsio/runtime';

/** The options of a file dialog. */
export interface OpenDialogOptions {
  title?: string;
  multiple?: boolean;
  directory?: boolean;
  defaultPath?: string;
  filters?: { name: string; extensions: string[] }[];
}

/** The options of a message dialog. */
export interface MessageDialogOptions {
  title?: string;
  message?: string;
  kind?: 'info' | 'warning' | 'error';
}

/** The options of a question dialog. */
export interface QuestionDialogOptions {
  title?: string;
  message: string;
  okLabel?: string;
  cancelLabel?: string;
}

/**
 * Opens a file dialog.
 *
 * The Tauri API returns `null` when the user cancels. The Wails one
 * returns an empty string or an empty array, which is translated here.
 */
export async function open(options: OpenDialogOptions = {}): Promise<string | string[] | null> {
  const result = await Dialogs.OpenFile({
    Title: options.title,
    AllowsMultipleSelection: options.multiple ?? false,
    CanChooseDirectories: options.directory ?? false,
    CanChooseFiles: !options.directory,
    Directory: options.defaultPath,
    Filters: options.filters?.map((filter) => ({
      DisplayName: filter.name,
      Pattern: filter.extensions.map((extension) => `*.${extension}`).join(';'),
    })),
  });

  if (Array.isArray(result)) {
    return result.length > 0 ? result : null;
  }
  return result ? result : null;
}

/** Opens a save dialog. */
export async function save(options: OpenDialogOptions = {}): Promise<string | null> {
  const result = await Dialogs.SaveFile({
    Title: options.title,
    Filename: options.defaultPath,
    Filters: options.filters?.map((filter) => ({
      DisplayName: filter.name,
      Pattern: filter.extensions.map((extension) => `*.${extension}`).join(';'),
    })),
  });
  return result ? result : null;
}

/**
 * Shows a message dialog.
 *
 * The Wails runtime has one dialog per kind, so the kind decides which
 * one is used. The buttons of the message dialog are the ones of the
 * platform.
 */
export async function message(
  message: string,
  options: MessageDialogOptions = { message },
): Promise<void> {
  const dialogOptions = {
    Title: options.title,
    Message: message,
    Buttons: [{ Label: 'OK', IsDefault: true }],
  };

  switch (options.kind) {
    case 'warning':
      await Dialogs.Warning(dialogOptions);
      return;
    case 'error':
      await Dialogs.Error(dialogOptions);
      return;
    default:
      await Dialogs.Info(dialogOptions);
  }
}

/**
 * Shows a question dialog and answers whether the user agreed.
 *
 * The dialog answers with the label of the button that was pressed, so
 * the label of the button that means "yes" is the one that is compared.
 */
export async function ask(
  message: string,
  options: QuestionDialogOptions = { message },
): Promise<boolean> {
  const okLabel = options.okLabel ?? 'OK';
  const cancelLabel = options.cancelLabel ?? 'Cancel';
  const result = await Dialogs.Question({
    Title: options.title,
    Message: message,
    Buttons: [
      { Label: okLabel, IsDefault: true },
      { Label: cancelLabel, IsCancel: true },
    ],
  });
  return result === okLabel;
}

/** Shows a confirmation dialog. */
export async function confirm(
  message: string,
  options: QuestionDialogOptions = { message },
): Promise<boolean> {
  return ask(message, options);
}
