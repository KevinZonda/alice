package campaign

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

func (s *Store) ListCampaigns(visibilityKey, statusFilter string, limit int) ([]Campaign, error) {
	if s == nil {
		return nil, errors.New("store is nil")
	}
	visibilityKey = strings.TrimSpace(visibilityKey)
	if visibilityKey == "" {
		return nil, errors.New("visibility key is empty")
	}
	statusFilter = strings.ToLower(strings.TrimSpace(statusFilter))
	if limit <= 0 {
		limit = defaultListLimit
	}
	if limit > maxListLimit {
		limit = maxListLimit
	}

	var campaigns []Campaign
	err := s.viewSnapshot(func(snapshot Snapshot) error {
		filtered := make([]Campaign, 0, len(snapshot.Campaigns))
		for _, raw := range snapshot.Campaigns {
			item := NormalizeCampaign(raw)
			if item.Session.VisibilityKey() != visibilityKey {
				continue
			}
			if statusFilter != "" && statusFilter != "all" && string(item.Status) != statusFilter {
				continue
			}
			filtered = append(filtered, item)
		}
		sort.Slice(filtered, func(i, j int) bool {
			left := filtered[i]
			right := filtered[j]
			if !left.CreatedAt.Equal(right.CreatedAt) {
				return left.CreatedAt.After(right.CreatedAt)
			}
			return left.ID > right.ID
		})
		if len(filtered) > limit {
			filtered = filtered[:limit]
		}
		campaigns = filtered
		return nil
	})
	if err != nil {
		return nil, err
	}
	return campaigns, nil
}

func (s *Store) ListAllCampaigns(statusFilter string, limit int) ([]Campaign, error) {
	if s == nil {
		return nil, errors.New("store is nil")
	}
	statusFilter = strings.ToLower(strings.TrimSpace(statusFilter))
	unlimited := limit < 0
	if !unlimited {
		if limit <= 0 {
			limit = defaultListLimit
		}
		if limit > maxListLimit {
			limit = maxListLimit
		}
	}

	var campaigns []Campaign
	err := s.viewSnapshot(func(snapshot Snapshot) error {
		filtered := make([]Campaign, 0, len(snapshot.Campaigns))
		for _, raw := range snapshot.Campaigns {
			item := NormalizeCampaign(raw)
			if statusFilter != "" && statusFilter != "all" && string(item.Status) != statusFilter {
				continue
			}
			filtered = append(filtered, item)
		}
		sort.Slice(filtered, func(i, j int) bool {
			left := filtered[i]
			right := filtered[j]
			if !left.CreatedAt.Equal(right.CreatedAt) {
				return left.CreatedAt.After(right.CreatedAt)
			}
			return left.ID > right.ID
		})
		if !unlimited && len(filtered) > limit {
			filtered = filtered[:limit]
		}
		campaigns = filtered
		return nil
	})
	if err != nil {
		return nil, err
	}
	return campaigns, nil
}

func (s *Store) GetCampaign(campaignID string) (Campaign, error) {
	if s == nil {
		return Campaign{}, errors.New("store is nil")
	}
	campaignID = strings.TrimSpace(campaignID)
	if campaignID == "" {
		return Campaign{}, errors.New("campaign id is empty")
	}

	var item Campaign
	err := s.viewSnapshot(func(snapshot Snapshot) error {
		idx := findCampaignIndex(snapshot.Campaigns, campaignID)
		if idx < 0 {
			return ErrCampaignNotFound
		}
		item = NormalizeCampaign(snapshot.Campaigns[idx])
		return nil
	})
	if err != nil {
		return Campaign{}, err
	}
	return item, nil
}

func (s *Store) DeleteCampaign(campaignID string) (Campaign, error) {
	if s == nil {
		return Campaign{}, errors.New("store is nil")
	}
	campaignID = strings.TrimSpace(campaignID)
	if campaignID == "" {
		return Campaign{}, errors.New("campaign id is empty")
	}

	var deleted Campaign
	err := s.updateSnapshot(func(snapshot *Snapshot) (bool, error) {
		idx := findCampaignIndex(snapshot.Campaigns, campaignID)
		if idx < 0 {
			return false, ErrCampaignNotFound
		}
		deleted = NormalizeCampaign(snapshot.Campaigns[idx])
		snapshot.Campaigns = append(snapshot.Campaigns[:idx], snapshot.Campaigns[idx+1:]...)
		return true, nil
	})
	if err != nil {
		return Campaign{}, err
	}
	return deleted, nil
}

func (s *Store) CreateCampaign(c Campaign) (Campaign, error) {
	if s == nil {
		return Campaign{}, errors.New("store is nil")
	}
	c = NormalizeCampaign(c)
	now := s.nowLocal()
	if c.ID == "" {
		c.ID = newID("camp", now)
	}
	if c.CreatedAt.IsZero() {
		c.CreatedAt = now
	}
	c.UpdatedAt = now
	if err := ValidateCampaign(c); err != nil {
		return Campaign{}, err
	}

	var created Campaign
	err := s.updateSnapshot(func(snapshot *Snapshot) (bool, error) {
		if findCampaignIndex(snapshot.Campaigns, c.ID) >= 0 {
			return false, fmt.Errorf("campaign id already exists: %s", c.ID)
		}
		snapshot.Campaigns = append(snapshot.Campaigns, c)
		created = c
		return true, nil
	})
	if err != nil {
		return Campaign{}, err
	}
	return created, nil
}

func (s *Store) PatchCampaign(campaignID string, mutate func(*Campaign) error) (Campaign, error) {
	if s == nil {
		return Campaign{}, errors.New("store is nil")
	}
	campaignID = strings.TrimSpace(campaignID)
	if campaignID == "" {
		return Campaign{}, errors.New("campaign id is empty")
	}
	if mutate == nil {
		return Campaign{}, errors.New("mutate callback is nil")
	}

	var updated Campaign
	err := s.updateSnapshot(func(snapshot *Snapshot) (bool, error) {
		idx := findCampaignIndex(snapshot.Campaigns, campaignID)
		if idx < 0 {
			return false, ErrCampaignNotFound
		}
		item := NormalizeCampaign(snapshot.Campaigns[idx])
		if err := mutate(&item); err != nil {
			return false, err
		}
		item = NormalizeCampaign(item)
		item.UpdatedAt = s.nowLocal()
		item.Revision++
		if err := ValidateCampaign(item); err != nil {
			return false, err
		}
		snapshot.Campaigns[idx] = item
		updated = item
		return true, nil
	})
	if err != nil {
		return Campaign{}, err
	}
	return updated, nil
}
