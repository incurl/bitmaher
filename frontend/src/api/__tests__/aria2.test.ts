/**
 * The download API at the Wails boundary.
 *
 * The tests check that the API calls the backend with the shapes the
 * backend declares, and that the answers are converted into the shapes
 * the ported components read.
 */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { createPinia, setActivePinia } from 'pinia';

const { service } = vi.hoisted(() => ({
  service: {
    AddDownload: vi.fn(),
    DownloadFiles: vi.fn(),
    DownloadStat: vi.fn(),
    ListDownloads: vi.fn(),
    PauseDownload: vi.fn(),
    PauseDownloadAll: vi.fn(),
    RemoveDownload: vi.fn(),
    ResumeDownload: vi.fn(),
    SelectDownloadFiles: vi.fn(),
    StopDownloadSharing: vi.fn(),
    SyncDownloads: vi.fn(),
  },
}));

const engineState = vi.hoisted(() => ({ isReady: false }));

vi.mock('../../../bindings/github.com/incurl/bitmaher/internal/bindings', () => ({
  AppService: service,
}));

vi.mock('@/stores/engine', () => ({
  useEngineStore: () => engineState,
}));

import {
  addTorrent,
  addUri,
  addUriAtomic,
  batchFinishSharing,
  changeOption,
  fetchTaskList,
  finishSharing,
  getFiles,
  getOption,
  pauseTask,
  refreshTasks,
  removeTask,
  resumeTask,
} from '../aria2';

describe('the download API', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.clearAllMocks();
    engineState.isReady = false;
    service.ListDownloads.mockResolvedValue([]);
    service.SyncDownloads.mockResolvedValue([]);
    service.DownloadStat.mockResolvedValue({
      download_speed: 0,
      upload_speed: 0,
      num_active: 0,
      num_waiting: 0,
      num_stopped: 0,
    });
  });

  describe('the task list', () => {
    it('refreshTasks reads the list and the statistics', async () => {
      service.SyncDownloads.mockResolvedValue([]);
      service.ListDownloads.mockResolvedValue([
        {
          gid: 'g1',
          status: 'active',
          total_length: 100,
          completed_length: 50,
          download_speed: 10,
          upload_speed: 0,
          connections: 2,
          dir: '/downloads',
        },
      ]);
      service.DownloadStat.mockResolvedValue({
        download_speed: 10,
        upload_speed: 5,
        num_active: 1,
        num_waiting: 0,
        num_stopped: 0,
      });

      const snapshot = await refreshTasks();

      expect(service.SyncDownloads).toHaveBeenCalled();
      expect(snapshot.stat.downloadSpeed).toBe(10);
      expect(snapshot.stat.numActive).toBe(1);
      expect(snapshot.tasks).toHaveLength(1);
    });

    it('converts the numbers of a task into strings', async () => {
      service.ListDownloads.mockResolvedValue([
        {
          gid: 'g1',
          status: 'active',
          total_length: 100,
          completed_length: 50,
          download_speed: 10,
          upload_speed: 0,
          connections: 2,
          dir: '/downloads',
        },
      ]);

      const tasks = await fetchTaskList({ type: 'all' });

      expect(tasks[0].totalLength).toBe('100');
      expect(tasks[0].completedLength).toBe('50');
      expect(tasks[0].downloadSpeed).toBe('10');
      expect(tasks[0].connections).toBe('2');
    });

    it('filters the tasks by their queue', async () => {
      service.ListDownloads.mockResolvedValue([
        { gid: 'a', status: 'active' },
        { gid: 'b', status: 'waiting' },
        { gid: 'c', status: 'complete' },
      ]);

      const active = await fetchTaskList({ type: 'active' });
      const stopped = await fetchTaskList({ type: 'stopped' });

      expect(active.map((task) => task.gid)).toEqual(['a']);
      expect(stopped.map((task) => task.gid)).toEqual(['c']);
    });
  });

  describe('the actions', () => {
    it('pauseTask pauses the download', async () => {
      await pauseTask({ gid: 'g1' });
      expect(service.PauseDownload).toHaveBeenCalledWith('g1');
    });

    it('resumeTask resumes the download', async () => {
      await resumeTask({ gid: 'g1' });
      expect(service.ResumeDownload).toHaveBeenCalledWith('g1');
    });

    it('removeTask removes the download', async () => {
      await removeTask({ gid: 'g1' });
      expect(service.RemoveDownload).toHaveBeenCalledWith('g1');
    });

    it('changeOption answers without a change', async () => {
      await changeOption({ gid: 'g1', options: { 'max-download-limit': '0' } });
      // The backend does not change the options of a running download,
      // so no call is expected.
      expect(service.RemoveDownload).not.toHaveBeenCalled();
    });
  });

  describe('the files of a download', () => {
    it('getFiles converts the files into the shape the components read', async () => {
      service.DownloadFiles.mockResolvedValue([
        {
          index: 1,
          path: '/downloads/movie.mkv',
          length: 1500000000,
          completed_length: 0,
          selected: true,
        },
      ]);

      const files = await getFiles({ gid: 'g1' });

      expect(service.DownloadFiles).toHaveBeenCalledWith('g1');
      expect(files).toHaveLength(1);
      expect(files[0].index).toBe('1');
      expect(files[0].path).toBe('/downloads/movie.mkv');
      expect(files[0].length).toBe('1500000000');
      expect(files[0].selected).toBe('true');
    });
  });

  describe('the sharing of a download', () => {
    it('finishSharing stops the sharing through the backend', async () => {
      // The call was a no-op: the backend had no such method, so the
      // user was told the sharing had stopped while it went on.
      service.StopDownloadSharing.mockResolvedValue(undefined);

      await finishSharing({ gid: 'g1' });

      expect(service.StopDownloadSharing).toHaveBeenCalledWith('g1');
    });

    it('batchFinishSharing stops every sharing and reports the failures', async () => {
      service.StopDownloadSharing.mockResolvedValueOnce(undefined).mockRejectedValueOnce(new Error('busy'));

      const result = await batchFinishSharing({ gids: ['g1', 'g2'] });

      expect(service.StopDownloadSharing).toHaveBeenCalledTimes(2);
      expect(result.succeeded).toEqual(['g1']);
      expect(result.failed).toHaveLength(1);
      expect(result.failed[0].gid).toBe('g2');
    });
  });

  describe('the options of a download', () => {
    it('getOption reads the options of the record', async () => {
      service.ListDownloads.mockResolvedValue([
        { gid: 'g1', options: { 'max-download-limit': '0' } },
      ]);

      const options = await getOption({ gid: 'g1' });

      expect(options).toEqual({ 'max-download-limit': '0' });
    });
  });

  describe('the creation of a download', () => {
    it('addUri starts one download and reports its gid', async () => {
      service.AddDownload.mockResolvedValue({ gid: 'gid1' });

      const gids = await addUri({
        uris: ['http://a.com/1.zip'],
        outs: [],
        options: { dir: '/downloads' },
      });

      expect(gids).toEqual(['gid1']);
      const request = service.AddDownload.mock.calls[0][0];
      expect(request.uri).toBe('http://a.com/1.zip');
      expect(request.kind).toBe('http');
      expect(request.dir).toBe('/downloads');
    });

    it('addUri recognises a magnet link', async () => {
      service.AddDownload.mockResolvedValue({ gid: 'gid1' });

      await addUri({
        uris: ['magnet:?xt=urn:btih:abc'],
        outs: [],
        options: {},
      });

      expect(service.AddDownload.mock.calls[0][0].kind).toBe('magnet');
    });

    it('addUriAtomic starts one download with every address', async () => {
      service.AddDownload.mockResolvedValue({ gid: 'gid1' });

      const gid = await addUriAtomic({
        uris: ['http://a.com/1.zip', 'http://b.com/1.zip'],
        options: {},
      });

      expect(gid).toBe('gid1');
      expect(service.AddDownload).toHaveBeenCalledTimes(1);
    });

    it('addTorrent sends the torrent file to the backend', async () => {
      service.AddDownload.mockResolvedValue({ gid: 'gid1' });

      const gid = await addTorrent({ torrent: 'base64data', options: {} });

      expect(gid).toBe('gid1');
      const request = service.AddDownload.mock.calls[0][0];
      expect(request.kind).toBe('torrent');
      expect(request.torrent_base64).toBe('base64data');
    });
  });
});
