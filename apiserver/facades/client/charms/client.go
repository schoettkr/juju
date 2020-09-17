// Copyright 2015 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package charms

import (
	"fmt"
	"io"

	"github.com/juju/charm/v8"
	"github.com/juju/collections/set"
	"github.com/juju/errors"
	"github.com/juju/juju/state"
	"github.com/juju/names/v4"
	"github.com/juju/utils"
	"gopkg.in/macaroon.v2"
	"gopkg.in/mgo.v2"

	apiservererrors "github.com/juju/juju/apiserver/errors"
	"github.com/juju/juju/apiserver/facade"
	"github.com/juju/juju/apiserver/params"
	"github.com/juju/juju/charmhub"
	corecharm "github.com/juju/juju/core/charm"
	"github.com/juju/juju/core/permission"
	stateerrors "github.com/juju/juju/state/errors"
	"github.com/juju/juju/state/storage"
	jujuversion "github.com/juju/juju/version"
)

// API implements the charms interface and is the concrete
// implementation of the API end point.
type API struct {
	authorizer   facade.Authorizer
	backendState BackendState
	backendModel BackendModel

	csResolverGetterFunc CSResolverGetterFunc
	getStrategyFunc      func(source string) StrategyFunc
	newStorage           func(modelUUID string, session *mgo.Session) storage.Storage
	tag                  names.ModelTag
}

type APIv2 struct {
	*API
}

func (a *API) checkCanRead() error {
	canRead, err := a.authorizer.HasPermission(permission.ReadAccess, a.tag)
	if err != nil {
		return errors.Trace(err)
	}
	if !canRead {
		return apiservererrors.ErrPerm
	}
	return nil
}

func (a *API) checkCanWrite() error {
	isAdmin, err := a.authorizer.HasPermission(permission.SuperuserAccess, a.backendState.ControllerTag())
	if err != nil {
		return errors.Trace(err)
	}

	canWrite, err := a.authorizer.HasPermission(permission.WriteAccess, a.tag)
	if err != nil {
		return errors.Trace(err)
	}
	if !canWrite && !isAdmin {
		return apiservererrors.ErrPerm
	}
	return nil
}

// NewFacadeV3 provides the signature required for facade V3 registration.
func NewFacadeV3(ctx facade.Context) (*API, error) {
	authorizer := ctx.Auth()
	if !authorizer.AuthClient() {
		return nil, apiservererrors.ErrPerm
	}

	st := ctx.State()
	m, err := st.Model()
	if err != nil {
		return nil, errors.Trace(err)
	}

	return &API{
		authorizer:           authorizer,
		backendState:         newStateShim(st),
		backendModel:         m,
		csResolverGetterFunc: csResolverGetter,
		getStrategyFunc:      getStrategyFunc,
		newStorage:           storage.NewStorage,
		tag:                  m.ModelTag(),
	}, nil
}

// NewFacade provides the signature required for facade V2 registration.
// It is unknown where V1 is.
func NewFacade(ctx facade.Context) (*APIv2, error) {
	v3, err := NewFacadeV3(ctx)
	if err != nil {
		return nil, nil
	}
	return &APIv2{v3}, nil
}

func NewCharmsAPI(
	authorizer facade.Authorizer,
	st BackendState,
	m BackendModel,
	csResolverFunc CSResolverGetterFunc,
	getStrategyFunc func(source string) StrategyFunc,
	newStorage func(modelUUID string, session *mgo.Session) storage.Storage,
) (*API, error) {
	return &API{
		authorizer:           authorizer,
		backendState:         st,
		backendModel:         m,
		csResolverGetterFunc: csResolverFunc,
		getStrategyFunc:      getStrategyFunc,
		newStorage:           newStorage,
		tag:                  m.ModelTag(),
	}, nil
}

// CharmInfo returns information about the requested charm.
// NOTE: thumper 2016-06-29, this is not a bulk call and probably should be.
func (a *API) CharmInfo(args params.CharmURL) (params.Charm, error) {
	logger.Tracef("CharmInfo %+v", args)
	if err := a.checkCanRead(); err != nil {
		return params.Charm{}, errors.Trace(err)
	}

	curl, err := charm.ParseURL(args.URL)
	if err != nil {
		return params.Charm{}, errors.Trace(err)
	}
	aCharm, err := a.backendState.Charm(curl)
	if err != nil {
		return params.Charm{}, errors.Trace(err)
	}
	info := params.Charm{
		Revision: aCharm.Revision(),
		URL:      curl.String(),
		Config:   params.ToCharmOptionMap(aCharm.Config()),
		Meta:     convertCharmMeta(aCharm.Meta()),
		Actions:  convertCharmActions(aCharm.Actions()),
		Metrics:  convertCharmMetrics(aCharm.Metrics()),
	}

	// we don't need to check that this is a charm.LXDProfiler, as we can
	// state that the function exists.
	if profile := aCharm.LXDProfile(); profile != nil && !profile.Empty() {
		info.LXDProfile = convertCharmLXDProfile(profile)
	}

	return info, nil
}

// List returns a list of charm URLs currently in the state.
// If supplied parameter contains any names, the result will
// be filtered to return only the charms with supplied names.
func (a *API) List(args params.CharmsList) (params.CharmsListResult, error) {
	logger.Tracef("List %+v", args)
	if err := a.checkCanRead(); err != nil {
		return params.CharmsListResult{}, errors.Trace(err)
	}

	charms, err := a.backendState.AllCharms()
	if err != nil {
		return params.CharmsListResult{}, errors.Annotatef(err, " listing charms ")
	}

	charmNames := set.NewStrings(args.Names...)
	checkName := !charmNames.IsEmpty()
	charmURLs := []string{}
	for _, aCharm := range charms {
		charmURL := aCharm.URL()
		if checkName {
			if !charmNames.Contains(charmURL.Name) {
				continue
			}
		}
		charmURLs = append(charmURLs, charmURL.String())
	}
	return params.CharmsListResult{CharmURLs: charmURLs}, nil
}

// AddCharm is not available via the V2 API.
func (a *APIv2) AddCharm(_ struct{}) {}

// AddCharm adds the given charm URL (which must include revision) to the
// environment, if it does not exist yet. Local charms are not supported,
// only charm store and charm hub URLs. See also AddLocalCharm().
func (a *API) AddCharm(args params.AddCharmWithOrigin) (params.CharmOriginResult, error) {
	logger.Tracef("AddCharm %+v", args)
	return a.addCharmWithAuthorization(params.AddCharmWithAuth{
		URL:                args.URL,
		Origin:             args.Origin,
		CharmStoreMacaroon: nil,
		Force:              args.Force,
	})
}

// AddCharmWithAuthorization is not available via the V2 API.
func (a *APIv2) AddCharmWithAuthorization(_ struct{}) {}

// AddCharmWithAuthorization adds the given charm URL (which must include
// revision) to the environment, if it does not exist yet. Local charms are
// not supported, only charm store and charm hub URLs. See also AddLocalCharm().
//
// The authorization macaroon, args.CharmStoreMacaroon, may be
// omitted, in which case this call is equivalent to AddCharm.
func (a *API) AddCharmWithAuthorization(args params.AddCharmWithAuth) (params.CharmOriginResult, error) {
	logger.Tracef("AddCharmWithAuthorization %+v", args)
	return a.addCharmWithAuthorization(args)
}

func (a *API) addCharmWithAuthorization(args params.AddCharmWithAuth) (params.CharmOriginResult, error) {
	if args.Origin.Source != "charm-hub" && args.Origin.Source != "charm-store" {
		return params.CharmOriginResult{}, errors.Errorf("unknown schema for charm URL %q", args.URL)
	}

	if err := a.checkCanWrite(); err != nil {
		return params.CharmOriginResult{}, err
	}

	strategy, err := a.charmStrategy(args)
	if err != nil {
		return params.CharmOriginResult{}, errors.Trace(err)
	}

	// Validate the strategy before running the download procedure.
	if err := strategy.Validate(); err != nil {
		return params.CharmOriginResult{}, errors.Trace(err)
	}

	defer func() {
		// Ensure we sign up any required clean ups.
		_ = strategy.Finish()
	}()

	// Run the strategy.
	result, alreadyExists, origin, err := strategy.Run(a.backendState, versionValidator{}, convertParamsOrigin(args.Origin))
	if err != nil {
		return params.CharmOriginResult{}, errors.Trace(err)
	} else if alreadyExists {
		// Nothing to do here, as it already exists in state.
		return params.CharmOriginResult{}, nil
	}

	ca := CharmArchive{
		ID:           strategy.CharmURL(),
		Charm:        result.Charm,
		Data:         result.Data,
		Size:         result.Size,
		SHA256:       result.SHA256,
		CharmVersion: result.Charm.Version(),
	}

	if args.CharmStoreMacaroon != nil {
		ca.Macaroon = macaroon.Slice{args.CharmStoreMacaroon}
	}

	OriginResult := params.CharmOriginResult{
		Origin: convertOrigin(origin),
	}

	// Store the charm archive in environment storage.
	if err = a.storeCharmArchive(ca); err != nil {
		OriginResult.Error = apiservererrors.ServerError(err)
	}

	return OriginResult, nil
}

type versionValidator struct{}

func (versionValidator) Validate(meta *charm.Meta) error {
	return jujuversion.CheckJujuMinVersion(meta.MinJujuVersion, jujuversion.Current)
}

// CharmArchive is the data that needs to be stored for a charm archive in
// state.
type CharmArchive struct {
	// ID is the charm URL for which we're storing the archive.
	ID *charm.URL

	// Charm is the metadata about the charm for the archive.
	Charm charm.Charm

	// Data contains the bytes of the archive.
	Data io.Reader

	// Size is the number of bytes in Data.
	Size int64

	// SHA256 is the hash of the bytes in Data.
	SHA256 string

	// Macaroon is the authorization macaroon for accessing the charmstore.
	Macaroon macaroon.Slice

	// Charm Version contains semantic version of charm, typically the output of git describe.
	CharmVersion string
}

// storeCharmArchive stores a charm archive in environment storage.
//
// TODO: (hml) 2020-09-01
// This is a duplicate of application.StoreCharmArchive.  Once use
// is transferred to this facade, it can be marked deprecated.
func (a *API) storeCharmArchive(archive CharmArchive) error {
	logger.Tracef("storeCharmArchive %q", archive.ID)
	storage := a.newStorage(a.backendState.ModelUUID(), a.backendState.MongoSession())
	storagePath, err := charmArchiveStoragePath(archive.ID)
	if err != nil {
		return errors.Annotate(err, "cannot generate charm archive name")
	}
	if err := storage.Put(storagePath, archive.Data, archive.Size); err != nil {
		return errors.Annotate(err, "cannot add charm to storage")
	}

	info := state.CharmInfo{
		Charm:       archive.Charm,
		ID:          archive.ID,
		StoragePath: storagePath,
		SHA256:      archive.SHA256,
		Macaroon:    archive.Macaroon,
		Version:     archive.CharmVersion,
	}

	// Now update the charm data in state and mark it as no longer pending.
	_, err = a.backendState.UpdateUploadedCharm(info)
	if err != nil {
		alreadyUploaded := err == stateerrors.ErrCharmRevisionAlreadyModified ||
			errors.Cause(err) == stateerrors.ErrCharmRevisionAlreadyModified ||
			stateerrors.IsCharmAlreadyUploadedError(err)
		if err := storage.Remove(storagePath); err != nil {
			if alreadyUploaded {
				logger.Errorf("cannot remove duplicated charm archive from storage: %v", err)
			} else {
				logger.Errorf("cannot remove unsuccessfully recorded charm archive from storage: %v", err)
			}
		}
		if alreadyUploaded {
			// Somebody else managed to upload and update the charm in
			// state before us. This is not an error.
			return nil
		}
		return errors.Trace(err)
	}
	return nil
}

// charmArchiveStoragePath returns a string that is suitable as a
// storage path, using a random UUID to avoid colliding with concurrent
// uploads.
func charmArchiveStoragePath(curl *charm.URL) (string, error) {
	uuid, err := utils.NewUUID()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("charms/%s-%s", curl.String(), uuid), nil
}

// ResolveCharms is not available via the V2 API.
func (a *APIv2) ResolveCharms(_ struct{}) {}

// ResolveCharms resolves the given charm URLs with an optionally specified
// preferred channel.  Channel provided via CharmOrigin.
func (a *API) ResolveCharms(args params.ResolveCharmsWithChannel) (params.ResolveCharmWithChannelResults, error) {
	logger.Tracef("ResolveCharms %+v", args)
	if err := a.checkCanRead(); err != nil {
		return params.ResolveCharmWithChannelResults{}, errors.Trace(err)
	}
	result := params.ResolveCharmWithChannelResults{
		Results: make([]params.ResolveCharmWithChannelResult, len(args.Resolve)),
	}
	for i, arg := range args.Resolve {
		result.Results[i] = a.resolveOneCharm(arg, args.Macaroon)
	}

	return result, nil
}

func (a *API) resolveOneCharm(arg params.ResolveCharmWithChannel, mac *macaroon.Macaroon) params.ResolveCharmWithChannelResult {
	result := params.ResolveCharmWithChannelResult{}
	curl, err := charm.ParseURL(arg.Reference)
	if err != nil {
		result.Error = apiservererrors.ServerError(err)
		return result
	}
	if !charm.CharmHub.Matches(curl.Schema) && !charm.CharmStore.Matches(curl.Schema) {
		result.Error = apiservererrors.ServerError(errors.Errorf("unknown schema for charm URL %q", curl.String()))
		return result
	}

	// If we can guarantee that each charm to be resolved uses the
	// same url source and channel, there is no need to get a new repository
	// each time.
	resolver, err := a.repository(arg.Origin, mac)
	if err != nil {
		result.Error = apiservererrors.ServerError(err)
		return result
	}

	resultURL, origin, supportedSeries, err := resolver.ResolveWithPreferredChannel(curl, arg.Origin)
	if err != nil {
		result.Error = apiservererrors.ServerError(err)
		return result
	}
	result.URL = resultURL.String()
	result.Origin = origin
	switch {
	case resultURL.Series != "" && len(supportedSeries) == 0:
		result.SupportedSeries = []string{resultURL.Series}
	default:
		result.SupportedSeries = supportedSeries
	}

	return result
}

func (a *API) charmStrategy(args params.AddCharmWithAuth) (Strategy, error) {
	repo, err := a.repository(args.Origin, args.CharmStoreMacaroon)
	if err != nil {
		return nil, err
	}
	strat := a.getStrategyFunc(args.Origin.Source)
	return strat(repo, args.URL, args.Force)
}

type StrategyFunc func(charmRepo corecharm.Repository, url string, force bool) (Strategy, error)

func getStrategyFunc(source string) StrategyFunc {
	if source == "charm-store" {
		return func(charmRepo corecharm.Repository, url string, force bool) (Strategy, error) {
			return corecharm.DownloadFromCharmStore(charmRepo, url, force)
		}
	}
	return func(charmRepo corecharm.Repository, url string, force bool) (Strategy, error) {
		return corecharm.DownloadFromCharmHub(charmRepo, url, force)
	}
}

func (a *API) repository(origin params.CharmOrigin, mac *macaroon.Macaroon) (corecharm.Repository, error) {
	switch origin.Source {
	case corecharm.CharmHub.String():
		return a.charmHubRepository()
	case corecharm.CharmStore.String():
		return a.charmStoreRepository(origin, mac)
	}
	return nil, errors.BadRequestf("Not charm hub nor charm store charm")
}

func (a *API) charmStoreRepository(origin params.CharmOrigin, mac *macaroon.Macaroon) (corecharm.Repository, error) {
	controllerCfg, err := a.backendState.ControllerConfig()
	if err != nil {
		return nil, errors.Trace(err)
	}
	client, err := a.csResolverGetterFunc(
		ResolverGetterParams{
			CSURL:              controllerCfg.CharmStoreURL(),
			Channel:            origin.Risk,
			CharmStoreMacaroon: mac,
		})
	if err != nil {
		return nil, errors.Trace(err)
	}
	return &csRepo{repo: client}, nil
}

func (a *API) charmHubRepository() (corecharm.Repository, error) {
	cfg, err := a.backendModel.Config()
	if err != nil {
		return nil, errors.Trace(err)
	}
	var chCfg charmhub.Config
	chURL, ok := cfg.CharmHubURL()
	if ok {
		chCfg, err = charmhub.CharmHubConfigFromURL(chURL, logger.Child("client"))
	} else {
		chCfg, err = charmhub.CharmHubConfig(logger.Child("client"))
	}
	if err != nil {
		return nil, errors.Trace(err)
	}

	chClient, err := charmhub.NewClient(chCfg)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return &chRepo{chClient}, nil
}

// IsMetered returns whether or not the charm is metered.
func (a *API) IsMetered(args params.CharmURL) (params.IsMeteredResult, error) {
	if err := a.checkCanRead(); err != nil {
		return params.IsMeteredResult{}, errors.Trace(err)
	}

	curl, err := charm.ParseURL(args.URL)
	if err != nil {
		return params.IsMeteredResult{Metered: false}, errors.Trace(err)
	}
	aCharm, err := a.backendState.Charm(curl)
	if err != nil {
		return params.IsMeteredResult{Metered: false}, errors.Trace(err)
	}
	if aCharm.Metrics() != nil && len(aCharm.Metrics().Metrics) > 0 {
		return params.IsMeteredResult{Metered: true}, nil
	}
	return params.IsMeteredResult{Metered: false}, nil
}
