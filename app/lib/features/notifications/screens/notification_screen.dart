// app/lib/features/notifications/screens/notification_screen.dart
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:intl/intl.dart';

import '../models/notification_job.dart';
import '../providers/notification_provider.dart';

final _dateFmt = DateFormat('dd MMM yyyy, HH:mm');

class NotificationScreen extends ConsumerStatefulWidget {
  const NotificationScreen({super.key});

  @override
  ConsumerState<NotificationScreen> createState() => _NotificationScreenState();
}

class _NotificationScreenState extends ConsumerState<NotificationScreen> {
  final _scrollController = ScrollController();

  @override
  void initState() {
    super.initState();
    // Load on first open.
    WidgetsBinding.instance.addPostFrameCallback((_) {
      ref.read(notificationProvider.notifier).refresh();
    });
    _scrollController.addListener(_onScroll);
  }

  void _onScroll() {
    if (_scrollController.position.pixels >=
        _scrollController.position.maxScrollExtent - 200) {
      ref.read(notificationProvider.notifier).fetchNext();
    }
  }

  @override
  void dispose() {
    _scrollController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(notificationProvider);

    return Scaffold(
      appBar: AppBar(
        title: const Text('Activity'),
        actions: [
          // Admin-only stale scan trigger.
          IconButton(
            icon: const Icon(Icons.refresh),
            tooltip: 'Trigger stale scan',
            onPressed: () => _triggerScan(context),
          ),
        ],
      ),
      body: RefreshIndicator(
        onRefresh: () => ref.read(notificationProvider.notifier).refresh(),
        child: _buildBody(state),
      ),
    );
  }

  Widget _buildBody(NotificationState state) {
    if (state.isLoading) {
      return const Center(child: CircularProgressIndicator());
    }

    if (state.error != null && state.jobs.isEmpty) {
      return Center(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(Icons.error_outline,
                  size: 48, color: Theme.of(context).colorScheme.error),
              const SizedBox(height: 12),
              Text(state.error!,
                  textAlign: TextAlign.center,
                  style: TextStyle(
                      color: Theme.of(context).colorScheme.error)),
              const SizedBox(height: 16),
              FilledButton(
                onPressed: () =>
                    ref.read(notificationProvider.notifier).refresh(),
                child: const Text('Retry'),
              ),
            ],
          ),
        ),
      );
    }

    if (state.jobs.isEmpty) {
      return const Center(
        child: Text('No notifications yet.',
            style: TextStyle(color: Colors.grey)),
      );
    }

    return ListView.builder(
      controller: _scrollController,
      itemCount: state.jobs.length + (state.isLoadingMore ? 1 : 0),
      itemBuilder: (context, i) {
        if (i >= state.jobs.length) {
          return const Padding(
            padding: EdgeInsets.all(16),
            child: Center(child: CircularProgressIndicator()),
          );
        }
        return _JobTile(job: state.jobs[i]);
      },
    );
  }

  Future<void> _triggerScan(BuildContext context) async {
    try {
      final msg =
          await ref.read(notificationProvider.notifier).triggerStaleScan();
      if (context.mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(msg)));
      }
    } catch (e) {
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
              content: Text('Scan failed: $e'),
              backgroundColor: Theme.of(context).colorScheme.error),
        );
      }
    }
  }
}

// ── Tile ──────────────────────────────────────────────────────────────────────

class _JobTile extends StatelessWidget {
  const _JobTile({required this.job});
  final NotificationJob job;

  @override
  Widget build(BuildContext context) {
    return ListTile(
      leading: _StatusIcon(status: job.status),
      title: Text(job.typeLabel),
      subtitle: Text(_dateFmt.format(job.createdAt.toLocal())),
      trailing: _StatusChip(status: job.status),
    );
  }
}

class _StatusIcon extends StatelessWidget {
  const _StatusIcon({required this.status});
  final String status;

  @override
  Widget build(BuildContext context) {
    final (icon, color) = switch (status) {
      'SENT'    => (Icons.check_circle_outline, Colors.green),
      'FAILED'  => (Icons.error_outline, Colors.red),
      _         => (Icons.schedule, Colors.orange),
    };
    return Icon(icon, color: color);
  }
}

class _StatusChip extends StatelessWidget {
  const _StatusChip({required this.status});
  final String status;

  @override
  Widget build(BuildContext context) {
    final (label, color) = switch (status) {
      'SENT'   => ('Sent', Colors.green),
      'FAILED' => ('Failed', Colors.red),
      _        => ('Pending', Colors.orange),
    };
    return Chip(
      label: Text(label,
          style: const TextStyle(fontSize: 11, color: Colors.white)),
      backgroundColor: color,
      padding: EdgeInsets.zero,
      materialTapTargetSize: MaterialTapTargetSize.shrinkWrap,
    );
  }
}
