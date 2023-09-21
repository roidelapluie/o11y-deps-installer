from __future__ import (absolute_import, division, print_function)
__metaclass__ = type

import json
import os
import os.path
from collections import defaultdict


from ansible.plugins.callback import CallbackBase

def aggregate_stats_to_dict(agg_stats):
    """Converts an AggregateStats object to a JSON-serializable dictionary, organized by hostname."""

    # Create a dictionary to hold all the information, initialized with defaultdict
    stats_dict = defaultdict(lambda: defaultdict(int))

    # List all status attributes
    status_attributes = ['processed', 'failures', 'ok', 'dark', 'changed', 'skipped', 'rescued', 'ignored']

    # Iterate over each status attribute in the object
    for attr in status_attributes:
        status_dict = getattr(agg_stats, attr, {})

        # Update the stats for each hostname
        for hostname, value in status_dict.items():
            stats_dict[hostname][attr] = value

    # Handle custom stats
    for hostname, custom_stats in agg_stats.custom.items():
        stats_dict[hostname]['custom'] = custom_stats

    # Convert defaultdict to regular dict for JSON serialization
    stats_dict = {k: dict(v) for k, v in stats_dict.items()}

    return stats_dict

class CallbackModule(CallbackBase):
    CALLBACK_VERSION = 2.0
    CALLBACK_TYPE = 'notification'
    CALLBACK_NAME = 'json_logger'
    CALLBACK_NEEDS_WHITELIST = True

    def __init__(self):
        super(CallbackModule, self).__init__()
        self.filename = os.environ.get('ANSIBLE_JSON_LOG_FILE', None)
        self.play_data = {}

    def update_existing_data(self, existing_data, new_data):
        for hostname, stats in new_data.items():
            if hostname not in existing_data:
                existing_data[hostname] = stats
            else:
                for key, value in stats.items():
                    if key in existing_data[hostname]:
                        existing_data[hostname][key] += value
                    else:
                        existing_data[hostname][key] = value

    def v2_playbook_on_stats(self, stats):
        if not self.filename:
            return

        if not os.path.exists(self.filename):
            with open(self.filename, 'w') as f:
                json.dump({}, f, indent=4)

        existing_data = {}
        try:
            with open(self.filename, 'r') as f:
                existing_data = json.load(f)
        except json.JSONDecodeError:
            existing_data = {}

        new_data = aggregate_stats_to_dict(stats)

        self.update_existing_data(existing_data, new_data)

        with open(self.filename, 'w') as f:
            json.dump(existing_data, f, indent=4)
